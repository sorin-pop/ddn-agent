FROM debian:stretch

RUN mkdir -p exports dumps
RUN apt-get update && apt-get install postgresql-client -y

ENV DDN_VENDOR=postgres DDN_EXEC=/usr/bin/psql

COPY sql /sql
COPY ddn-agent /

ENTRYPOINT ["/ddn-agent"]
