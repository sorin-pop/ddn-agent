FROM debian:stretch

WORKDIR /ddn

RUN mkdir -p exports dumps data /usr/share/man/man1
RUN apt-get update && apt-get install postgresql-client -y

COPY agent ./
COPY sql ./sql

VOLUME ["/ddn/data"]

EXPOSE 7000

ENTRYPOINT ["/ddn/agent", "-p", "/ddn/data/agent.conf", "-l", "/ddn/data/agent.log"]
