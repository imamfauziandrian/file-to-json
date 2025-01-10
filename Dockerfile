FROM golang:alpine as build
WORKDIR /app
COPY . .
RUN go build -o /server .

FROM scratch
COPY --from=build /server server
EXPOSE 7878
CMD ["/server"]