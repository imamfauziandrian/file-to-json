FROM golang:1.23 as build
WORKDIR /app
COPY . .
RUN go build -o /server .

FROM scratch
COPY --from=build /app/server /server
EXPOSE 7878
CMD ["/server"]