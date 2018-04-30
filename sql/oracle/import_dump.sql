WHENEVER OSERROR EXIT FAILURE
WHENEVER SQLERROR EXIT SQL.SQLCODE
SET VERIFY OFF
SET SERVEROUTPUT ON
SET FEEDBACK OFF

var  dump_dir VARCHAR2(2000);
var  target_schema VARCHAR2(50);

EXECUTE :dump_dir := '&1';
EXECUTE :target_schema := '&3';

EXECUTE import_dump(:dump_dir, '&2', :target_schema , '&4', '&5');

EXIT
