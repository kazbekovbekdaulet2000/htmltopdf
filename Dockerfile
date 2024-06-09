
FROM docker.io/golang:1.17.2
WORKDIR /tmp
COPY main.go .
RUN go build main.go

FROM docker.io/ubuntu:18.04

RUN set -x \
  && apt update \
  && apt -y install --no-install-recommends wget ca-certificates fonts-liberation2 \
  && wget -q -O /tmp/wkhtmltox.deb https://github.com/wkhtmltopdf/wkhtmltopdf/releases/download/0.12.5/wkhtmltox_0.12.5-1.bionic_amd64.deb \
  && echo "f1689a1b302ff102160f2693129f789410a1708a /tmp/wkhtmltox.deb" | sha1sum -c - \
  && apt -y install /tmp/wkhtmltox.deb \
  && apt -y purge wget --autoremove \
  && apt -y clean \
  && rm -rf /var/lib/apt/lists/* \
  && rm /tmp/wkhtmltox.deb

COPY --from=0 /tmp/wkhtmltopdf /usr/local/bin/

RUN adduser --disabled-password --gecos '' wkhtmltopdf
USER wkhtmltopdf
ENV LANG C.UTF-8
ENV LC_ALL C.UTF-8

EXPOSE 8080
CMD /usr/local/bin/wkhtmltopdf
