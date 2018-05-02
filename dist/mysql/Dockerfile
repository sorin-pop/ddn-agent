FROM debian:stretch-slim

RUN mkdir -p exports dumps
RUN apt-get update && apt-get install mysql-client -y

ENV DDN_VENDOR=mysql DDN_EXEC=/usr/bin/mysql

COPY sql /sql
COPY ddn-agent /

ENTRYPOINT ["/ddn-agent"]
