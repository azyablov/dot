FROM golang:1.16

WORKDIR /go/src/app
COPY . .

ENV DNSSERVER "8.8.8.8"
ENV LISPORT "853"
EXPOSE 853/tcp

CMD ["./dotstart.sh"]