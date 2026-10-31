FROM alpine:latest
WORKDIR /root/
COPY myapp .
EXPOSE 8080
CMD ["./myapp"]