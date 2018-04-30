FROM debian:stretch-slim

WORKDIR /ddn

RUN mkdir -p exports dumps data
RUN apt-get update && apt-get install mysql-client -y

COPY agent ./
COPY sql ./sql

VOLUME ["/ddn/data"]

EXPOSE 7000

ENTRYPOINT ["/ddn/agent", "-p", "/ddn/data/agent.conf", "-l", "/ddn/data/agent.log"]
