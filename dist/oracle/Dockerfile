FROM centos:7

RUN yum install unzip libaio -y
ADD instantclient-basic-linux.x64-11.2.0.4.0.zip instantclient-sqlplus-linux.x64-11.2.0.4.0.zip /

RUN unzip -q instantclient-basic-linux.x64-11.2.0.4.0.zip && unzip -q instantclient-sqlplus-linux.x64-11.2.0.4.0.zip && rm instantclient-basic-linux.x64-11.2.0.4.0.zip instantclient-sqlplus-linux.x64-11.2.0.4.0.zip

ENV LD_LIBRARY_PATH=/instantclient_11_2 DDN_VENDOR=oracle DDN_EXEC=/instantclient_11_2/sqlplus

COPY sql /sql
COPY ddn-agent /

CMD /ddn-agent
