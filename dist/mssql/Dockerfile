FROM mcr.microsoft.com/mssql-tools

ENV DDN_VENDOR=mssql DDN_EXEC=/opt/mssql-tools/bin/sqlcmd

COPY sql /sql
COPY ddn-agent /

CMD /ddn-agent
