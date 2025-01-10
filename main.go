package main

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/jlaffaye/ftp"
	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type ExcelToJsonParam struct {
	Source          string `query:"source"` // url, ftp, file
	Url             string `query:"url"`
	Username        string `query:"username"`
	Password        string `query:"password"`
	FileName        string `query:"file_name"`
	FileNameContain string `query:"file_name_contain"`
}

func main() {
	app := fiber.New(fiber.Config{
		JSONEncoder: sonic.Marshal,
		JSONDecoder: sonic.Unmarshal,
	})

	app.Use(compress.New())

	app.Get("/excel-to-json", func(c *fiber.Ctx) error {
		// set memory limit
		// debug.SetMemoryLimit(2000 * 1024 * 1024)

		p := new(ExcelToJsonParam)
		if err := c.QueryParser(p); err != nil {
			return err
		}

		data := make([]map[string]string, 0)
		loadExcel := func(r io.Reader) error {
			// Read Excel file
			f, err := excelize.OpenReader(r)
			if err != nil {
				fmt.Println(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to read file.")
			}
			defer func() {
				// Close the spreadsheet.
				if err := f.Close(); err != nil {
					fmt.Println(err)
				}
			}()

			// Get all the rows in the Sheet1.
			rows, err := f.GetRows(f.GetSheetName(0), excelize.Options{RawCellValue: true})
			if err != nil {
				fmt.Println(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to get rows.")
			}

			headers := rows[0]
			data = make([]map[string]string, len(rows)-1)
			for i, row := range rows[1:] {
				data[i] = make(map[string]string)
				for j, cellValue := range row {
					data[i][headers[j]] = cellValue
				}
			}

			return nil
		}

		if p.Source == "ftp" {
			if len(p.Url) == 0 {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid or empty url parameter.")
			}

			if len(p.Username) == 0 {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid or empty username parameter.")
			}

			if len(p.Password) == 0 {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid or empty password parameter.")
			}

			// Load from FTP
			ftpConn, err := ftp.Dial(p.Url, ftp.DialWithTimeout(5*time.Second))
			if err != nil {
				log.Fatal(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to connect url.")
			}

			err = ftpConn.Login(p.Username, p.Password)
			if err != nil {
				log.Fatal(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to login using given username and password.")
			}

			ls, err := ftpConn.List("/")
			if err != nil {
				log.Fatal(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to fetch list file.")
			}
			if len(ls) == 0 {
				return fiber.NewError(fiber.StatusBadRequest, "No files or directories found in the FTP storage.")
			}

			ff := lo.Filter(ls, func(x *ftp.Entry, index int) bool {
				return strings.Contains(x.Name, p.FileNameContain)
			})
			if len(ff) == 0 {
				return fiber.NewError(fiber.StatusBadRequest, "No document matching your search criteria was found.")
			}

			sort.Slice(ff, func(a, b int) bool {
				return ff[b].Time.Before(ff[a].Time)
			})

			r, err := ftpConn.Retr(ff[0].Name)
			if err != nil {
				log.Fatal(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to download file.")
			}
			defer r.Close()

			if err := loadExcel(r); err != nil {
				log.Fatal(err)
				return err
			}

			if err := ftpConn.Quit(); err != nil {
				log.Fatal(err)
				return fiber.NewError(fiber.StatusBadRequest, "Failed to disconnect from source url.")
			}
		}

		defer func() {
			// clear memory
			// debug.SetMemoryLimit(20 * 1024 * 1024)
		}()

		return c.JSON(data)
	})

	app.Listen(":7878")
}
