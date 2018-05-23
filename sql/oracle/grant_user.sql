WHENEVER OSERROR EXIT FAILURE
WHENEVER SQLERROR EXIT SQL.SQLCODE

@$ORACLE_HOME/rdbms/admin/catexf.sql

grant select on dba_datapump_jobs to &1;
grant create any directory to &1;
grant create EXTERNAL JOB to &1;
grant execute on SYS.utl_file to &1;

EXIT
