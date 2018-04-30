-- Prerequisites. The below are needed for the procedure to compile successfully
-- conn sys as sysdba
-- grant select on dba_datapump_jobs to system;
-- grant create any directory to system;
-- grant create external job to system;

WHENEVER OSERROR EXIT FAILURE
WHENEVER SQLERROR EXIT SQL.SQLCODE

CREATE OR REPLACE PROCEDURE import_dump(dump_dir IN OUT varchar2, fn IN varchar2, ts IN OUT varchar2, ts_pwd IN varchar2, datafile_dir IN varchar2) AUTHID CURRENT_USER AS
jn    varchar2(256);
o_schema varchar2(256);
dump_file varchar2(256);
o_tablespace varchar2(256);
o_schemas varchar2(1000) :='';
exported_schemas dbms_sql.varchar2_table;
exported_tablespaces dbms_sql.varchar2_table;
dump_files dbms_sql.varchar2_table;

info  ku$_dumpfile_info;
ft    number;

handle1   number;
js  dba_datapump_jobs.state%type;
query_str VARCHAR2(1000);
c SYS_REFCURSOR;
i number := 0;
dump_file_iterator number := 0;
import_start timestamp;
import_stop timestamp;
duration INTERVAL DAY TO SECOND;
duration_days NUMBER;
duration_hours NUMBER;
duration_minutes NUMBER;
duration_seconds NUMBER;
days_string VARCHAR2(100) := '';
hours_string VARCHAR2(100) := '';
minutes_string VARCHAR2(100) := '';
seconds_string VARCHAR2(100) := '';
master_table_present VARCHAR2(2048);
metadata_encrypted VARCHAR2(2048);
data_encrypted	VARCHAR2(2048);
source_db_version VARCHAR2(2048);

job_doesnt_exist EXCEPTION;
PRAGMA EXCEPTION_INIT( job_doesnt_exist, -27475 );
err_msg VARCHAR2(200);

object_file_name VARCHAR2(2048);
object_file UTL_FILE.FILE_TYPE;
new_object_file UTL_FILE.FILE_TYPE;
object_file_line VARCHAR2(32767);
new_object_file_line VARCHAR2(32767);
ends_with_tablespace BOOLEAN;
tablespace_position NUMBER;
tablespace_name_start NUMBER;
tablespace_name_end NUMBER;
dump_tablespace VARCHAR2(2048);
schema_name_start NUMBER;
schema_name_end NUMBER;
dump_schema VARCHAR2(2048);
datafile1 VARCHAR2(2000);
datafile2 VARCHAR2(2000);
file_clause VARCHAR2(4000) :='';
  
BEGIN
	IF (SUBSTR(dump_dir, -1, 1) = '/') OR (SUBSTR(dump_dir, -1, 1) = '\') THEN   --chr(92)
		dump_dir := SUBSTR(dump_dir,1,LENGTH(dump_dir) - 1);
	END IF;	
	query_str := 'CREATE OR REPLACE DIRECTORY DATA_PUMP_DIR AS ' || '''' || dump_dir || '''';
	EXECUTE IMMEDIATE query_str;
	
	ts := UPPER(ts);
	BEGIN
		query_str := 'DROP USER ' || ts || ' CASCADE';
		EXECUTE IMMEDIATE query_str;

		query_str := 'DROP TABLESPACE ' || ts || ' INCLUDING CONTENTS AND DATAFILES';
		EXECUTE IMMEDIATE query_str;
	EXCEPTION
    WHEN OTHERS THEN
		IF SQLCODE = -1918 THEN -- if target schema does not exists, we swallow the exception, do nothing
			NULL;
		END IF;
		IF SQLCODE = -959 THEN -- if the tablespace of the user does not exists, we swallow the exception, do nothing
			NULL;
		END IF;
	END;

	datafile1 := datafile_dir || ts || '_01.dbf';
	datafile2 := datafile_dir || ts || '_02.dbf';
	
	-- we first create a separate tablespace for the user (see https://github.com/djavorszky/ddn/issues/154)
	query_str := 'CREATE SMALLFILE TABLESPACE ' || ts || ' DATAFILE ' || '''' || datafile1 || '''' || ' SIZE 32M AUTOEXTEND ON MAXSIZE UNLIMITED';
	EXECUTE IMMEDIATE query_str;
	query_str := 'ALTER TABLESPACE ' || ts || ' ADD DATAFILE ' || '''' || datafile2 || '''' || ' SIZE 1M AUTOEXTEND ON MAXSIZE UNLIMITED';
	EXECUTE IMMEDIATE query_str;
	
	query_str := 'create user ' || ts || ' identified by ' || ts_pwd || ' default tablespace ' || ts || ' quota unlimited on ' || ts;
	EXECUTE IMMEDIATE query_str;
	query_str := 'grant connect, resource to ' || ts;
	EXECUTE IMMEDIATE query_str;
	
	-- we populate the dump file table (necessary when the export was made to multiple dump files)
	-- multiple dump files can be provided in the fn parameter by delimiting them with commas. e.g 'file1.dmp,file2.dmp,file3.dmp'
	
	query_str := 'select regexp_substr(' || '''' || fn || '''' || ',' || '''' || '[^,]+' || '''' || ', 1, level) from dual connect by regexp_substr(' || '''' || fn || '''' || ',' || '''' || '[^,]+' || '''' || ', 1, level) is not null';
		
	OPEN c FOR query_str;
	LOOP
		FETCH c INTO dump_file;
		EXIT WHEN c%NOTFOUND;
		dump_file_iterator := dump_file_iterator + 1;
		dump_files(dump_file_iterator) := dump_file;
	END LOOP;
	CLOSE c;	
	
	dump_file_iterator := 1;
	
	<<read_dumpfile>>
	sys.dbms_datapump.get_dumpfile_info(
                                      dump_files(dump_file_iterator),
                                      'DATA_PUMP_DIR',
                                      info,
                                      ft
	);

	if ft = 0 then
		dbms_output.put_line('Dump file type not recognized.');
	elsif ft = 1 then
		dbms_output.put_line('Dump file is a DataPump export file.');
		
		-- check if master table is present in the file
		-- if it is not, chances are this is part of a multiple dump file set
		-- therefore we write out this information and we stop, we don't continue with loading and querying the master table
		BEGIN
			select value into master_table_present from TABLE(info) where item_code = DBMS_DATAPUMP.KU$_DFHDR_MASTER_PRESENT;
			IF master_table_present = '0' THEN
				dbms_output.put_line('Master table is not present in the dump file ' || dump_files(dump_file_iterator) || '. Retrying with the next dump file in the set.');
				dump_file_iterator := dump_file_iterator + 1;
				GOTO read_dumpfile;
			END IF;
		EXCEPTION
			WHEN NO_DATA_FOUND THEN
				err_msg := 'Could not determine if dump file ' || dump_files(dump_file_iterator) || ' contains the master table.';
				err_msg := err_msg || chr(10) || 'This could be due to the fact that you are running this script on a different major version of Oracle than the one the dump originates from.';
				dbms_output.put_line(err_msg);
				raise_application_error(-20000, err_msg);
		END;
		
		-- is it encrypted?(possible only if the export was done from at least Oracle 11g)
		BEGIN
			select value into source_db_version from TABLE(info) where item_code = DBMS_DATAPUMP.KU$_DFHDR_DB_VERSION;
			IF SUBSTR(source_db_version,1,INSTR(source_db_version,'.')-1) >= '11' THEN
				select value into metadata_encrypted from TABLE(info) where item_code = DBMS_DATAPUMP.KU$_DFHDR_METADATA_ENCRYPTED;
				select value into data_encrypted from TABLE(info) where item_code = DBMS_DATAPUMP.KU$_DFHDR_DATA_ENCRYPTED;
				dbms_output.put_line('Metadata encrypted: ' || metadata_encrypted);
				dbms_output.put_line('Data encrypted: ' || data_encrypted);
				IF NOT (metadata_encrypted = '0' AND data_encrypted = '0') THEN
					-- dump is encrypted
					dbms_output.put_line('Dump file is encrypted!');
					raise_application_error(-20000, 'Dump is encrypted! Handling encrypted dumps is currently not a feature in CloudDB. Please contact CloudDB admins for further help.');
				END IF;
			END IF;	
		EXCEPTION
			WHEN NO_DATA_FOUND THEN
				err_msg := 'Could not determine source database version or could not determine whether dump is encrypted or not.';
				err_msg := err_msg || chr(10) || 'This could be due to the fact that you are running this script on a different major version of Oracle than the one the dump originates from.';
				dbms_output.put_line(err_msg);
				raise_application_error(-20000, err_msg);
		END;
		
		-- list full info contents
		/*FOR i IN 1 .. info.COUNT LOOP
			dbms_output.put_line(i || '      ' || info(i).item_code || '         ' || info(i).value);
		END LOOP;*/
		
		dbms_output.put_line('Extracting schema/tablespace info...');
	
		-- load master table
		jn := SUBSTR(REPLACE('READ_' || dump_files(dump_file_iterator), '.','_'),1,30);
		handle1 := dbms_datapump.open (operation => 'IMPORT', job_mode => 'FULL', job_name => jn); 
		dbms_datapump.add_file(handle => handle1, filename => dump_files(dump_file_iterator), directory => 'DATA_PUMP_DIR', filetype => DBMS_DATAPUMP.KU$_FILE_TYPE_DUMP_FILE);
		dbms_datapump.set_parameter(handle => handle1, name => 'MASTER_ONLY', value => 1);
		dbms_datapump.set_parameter(handle => handle1, name => 'KEEP_MASTER', value => 1);
		dbms_datapump.start_job(handle => handle1); 
		dbms_datapump.wait_for_job(handle => handle1, job_state => js); 
		dbms_output.put_line('Loading of the master table has been ' || js);
		dbms_output.put_line(chr(10));
	
		-- read the loaded master table (has the same name as the job name) and then drop it
		
		--schemas
		query_str := 'select distinct'
			|| ' object_schema'
			|| ' from ' || '"' || jn || '"'
			|| ' where process_order > 0 and OBJECT_TYPE=''TABLE_DATA'''
			|| ' and object_name is not null'
			|| ' and object_schema not in (''SYSTEM'',''SYS'',''OUTLN'',''ORDDATA'',''FLOWS_FILES'',''APEX_PUBLIC_USER'',''SYSMAN'',''OWBSYS'',''OLAPSYS'')';
		
		dbms_output.put_line(rpad('Schemas',30));	
		dbms_output.put_line('------------------------------');	

		i := 0;
		OPEN c FOR query_str;
		LOOP
			FETCH c INTO o_schema;
			EXIT WHEN c%NOTFOUND;
			i := i + 1;
			exported_schemas(i) := o_schema;
			o_schemas := o_schemas || ',' || '''' || o_schema || '''';
			dbms_output.put_line(rpad(o_schema,30));	
		END LOOP;
		CLOSE c;
		dbms_output.put_line('------------------------------');	
		dbms_output.put_line(chr(10));	


		
		-- tablespaces
		query_str := 'select distinct'
			||' object_tablespace'
			|| ' from ' || '"' || jn || '"'
			|| ' where process_order > 0 and OBJECT_TYPE=''TABLE_DATA'''
			|| ' and object_name is not null'
			|| ' and object_schema not in (''SYSTEM'',''SYS'',''OUTLN'',''ORDDATA'',''FLOWS_FILES'',''APEX_PUBLIC_USER'',''SYSMAN'',''OWBSYS'',''OLAPSYS'')';
			
		dbms_output.put_line(rpad('Tablespaces',30));	
		dbms_output.put_line('------------------------------');	

		i := 0;
		OPEN c FOR query_str;
		LOOP
			FETCH c INTO o_tablespace;
			EXIT WHEN c%NOTFOUND;
			i := i + 1;
			exported_tablespaces(i) := o_tablespace;
			dbms_output.put_line(rpad(o_tablespace,30));	
		END LOOP;
		CLOSE c;
		dbms_output.put_line('------------------------------');	
		dbms_output.put_line(chr(10));	
			
		EXECUTE IMMEDIATE 'DROP TABLE ' || '"' || jn || '"';
		
		
		-- start a datapump job to import the dump file
		handle1 := dbms_datapump.open (operation => 'IMPORT', job_mode => 'FULL', job_name => ts);
		dbms_datapump.add_file(handle => handle1, filename => ts || '.LOG', directory => 'DATA_PUMP_DIR', filetype => DBMS_DATAPUMP.KU$_FILE_TYPE_LOG_FILE); 
		dbms_datapump.metadata_filter(handle => handle1, name => 'SCHEMA_EXPR', value => 'IN(' || LTRIM(o_schemas,',') || ')');
		
		FOR i IN dump_files.FIRST .. dump_files.LAST
		LOOP
			dbms_datapump.add_file(handle => handle1, filename => dump_files(i), directory => 'DATA_PUMP_DIR');
		END LOOP;
		
		FOR i IN exported_schemas.FIRST .. exported_schemas.LAST
		LOOP
			dbms_datapump.metadata_remap(handle => handle1, name => 'REMAP_SCHEMA', old_value => exported_schemas(i), value => ts ); 
		END LOOP;
		
		FOR i IN exported_tablespaces.FIRST .. exported_tablespaces.LAST
		LOOP	
			dbms_datapump.metadata_remap(handle => handle1, name => 'REMAP_TABLESPACE', old_value => exported_tablespaces(i), value => ts );
		END LOOP;	
		
		import_start := CURRENT_TIMESTAMP;
		dbms_datapump.start_job(handle => handle1);
		dbms_datapump.wait_for_job(handle => handle1, job_state => js);
		
		import_stop := CURRENT_TIMESTAMP;
		dbms_output.put_line('Import of ' || fn || ' has been ' || js);
		duration := import_stop - import_start;
		duration_days := EXTRACT(DAY FROM duration);
		duration_hours := EXTRACT(HOUR FROM duration);
		duration_minutes := EXTRACT(MINUTE FROM duration);
		duration_seconds := ROUND(EXTRACT(SECOND FROM duration));
		if duration_days > 0 then
			if duration_days > 1 then
				days_string := duration_days || ' days, ';
			else
				days_string := duration_days || ' day, ';
			end if;	
		end if;
		if duration_hours > 0 then
			if duration_hours > 1 then
				hours_string := ', ' || duration_hours || ' hours';
			else
				hours_string := ', ' || duration_hours || ' hour';
			end if;	
		end if;
		if duration_minutes > 0 then
			if duration_minutes > 1 then
				minutes_string := ', ' || duration_minutes || ' minutes';
			else
				minutes_string := ', ' || duration_minutes || ' minute';
			end if;	
		else
			if duration_seconds = 0 then
				hours_string := REPLACE(hours_string, ',', ' and');
			end if;	
		end if;
		if duration_seconds > 0 then
			if duration_seconds > 1 then
				seconds_string := ' and ' || duration_seconds || ' seconds';
			else
				seconds_string := ' and ' || duration_seconds || ' second';
			end if;
		else
			minutes_string := REPLACE(minutes_string, ',', ' and');
		end if;
		dbms_output.put_line('Import job ran for ' || days_string || hours_string || minutes_string || seconds_string || '.');
	elsif ft = 2 then
		dbms_output.put_line('Dump file is a Classic export file.');
		EXECUTE IMMEDIATE 'GRANT IMP_FULL_DATABASE TO ' || ts;

		-- 1. first only generate <ts>_objects.sql, which contains the object creation commands
		-- imp ts/ts_pwd file=fn  full=y log=jn.log indexfile=create.sql
		jn := SUBSTR(ts || '_objects',1,30);
		object_file_name := ts || '_objects.sql';
		DBMS_SCHEDULER.CREATE_JOB(JOB_NAME => jn,JOB_TYPE => 'EXECUTABLE',JOB_ACTION =>'imp', NUMBER_OF_ARGUMENTS => 5);
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 1, ts || '/' || ts_pwd);
		
		FOR i IN dump_files.FIRST .. dump_files.LAST
		LOOP
			file_clause := file_clause || dump_dir || '/' ||  dump_files(i) || ',';
		END LOOP;
		-- remove the last comma
		file_clause := SUBSTR(file_clause,1,LENGTH(file_clause) - 1);
		file_clause := 'file="' || file_clause || '"';
		
		--DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 2, 'file="' || dump_dir || '/' ||  fn || '"');
		--dbms_output.put_line(file_clause);
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 2, file_clause);
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 3, 'full=Y');
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 4, 'log="' || dump_dir || '/' || jn || '.log' || '"');
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 5, 'indexfile="' || dump_dir || '/' || object_file_name || '"');
		DBMS_SCHEDULER.RUN_JOB(JOB_NAME => jn, USE_CURRENT_SESSION => TRUE);
		DBMS_SCHEDULER.DROP_JOB(jn);
		
		-- 2. Edit <ts>_objects.sql (remove 'REM  ' from line beginnings, remove lines starting with 'REM  ...', 
		-- replace tablespace name everywhere with our target tablespace, remove the line starting with "CONNECT ",
		-- remove the schema name in CREATE/ALTER TABLE and CREATE (UNIQUE) INDEX commands, as it turned out that it can be inconsistent - see DDN-263)
		object_file := UTL_FILE.FOPEN('DATA_PUMP_DIR', object_file_name,'R',32767);
		new_object_file := UTL_FILE.FOPEN('DATA_PUMP_DIR', 'new_' || object_file_name,'W',32767);
		ends_with_tablespace := FALSE;
		UTL_FILE.PUT_LINE(new_object_file, 'SPOOL ' || dump_dir || '/run_' || REPLACE(object_file_name,'.sql','.log'), TRUE);
		-- read object_file line by line
		LOOP
			BEGIN
			  UTL_FILE.GET_LINE(object_file, object_file_line);
			EXCEPTION
			  WHEN NO_DATA_FOUND THEN
				EXIT;
			END;
			
			-- process line
			new_object_file_line := object_file_line;
			
			-- remove 'REM  ' from line beginnings, completely ignore lines starting with 'REM  ...'
			IF SUBSTR(new_object_file_line, 1, 5) = 'REM  ' THEN
				IF SUBSTR(new_object_file_line, 1, 8) = 'REM  ...' THEN
					CONTINUE;
				ELSE
					new_object_file_line := SUBSTR(new_object_file_line,6);
				END IF;
			END IF;

			-- ignore the CONNECT line, we won't need it, and it doesn't have the password anyway. 
			IF UPPER(new_object_file_line) LIKE 'CONNECT %' THEN
				CONTINUE;
			END IF;

			-- remove the schema name in CREATE/ALTER TABLE and CREATE (UNIQUE) INDEX commands
			IF new_object_file_line LIKE 'CREATE TABLE %' OR
				new_object_file_line LIKE 'ALTER TABLE %' OR
				new_object_file_line LIKE 'CREATE INDEX %' OR
				new_object_file_line LIKE 'CREATE UNIQUE INDEX %' THEN

					schema_name_start := INSTR(new_object_file_line, '"', 1, 1) ;
					schema_name_end := INSTR(new_object_file_line, '"', 1, 2);
					dump_schema := SUBSTR(new_object_file_line, schema_name_start, schema_name_end - schema_name_start + 2);
					new_object_file_line := REPLACE(new_object_file_line, dump_schema);
			END IF;
			
			-- if TABLESPACE keyword present in the line, replace tablespace name with our target tablespace
			tablespace_position := INSTR(new_object_file_line, 'TABLESPACE');
			IF tablespace_position != 0 THEN
				-- if it ends in the TABLESPACE word, we'll replace the tablespace name in the next line
				IF SUBSTR(new_object_file_line, -11) LIKE '%TABLESPACE%' THEN
					ends_with_tablespace := TRUE;
				ELSE
					ends_with_tablespace := FALSE;
					tablespace_name_start := INSTR(new_object_file_line, '"', tablespace_position, 1) + 1 ;
					tablespace_name_end := INSTR(new_object_file_line, '"', tablespace_position, 2);
					dump_tablespace := SUBSTR(new_object_file_line, tablespace_name_start, tablespace_name_end - tablespace_name_start);
					new_object_file_line := REPLACE(new_object_file_line, dump_tablespace, ts);
				END IF;
			ELSE
				-- if previous line ended with the TABLESPACE word, this line will start with the tablespace name. We replace it. 
				IF ends_with_tablespace = TRUE THEN
					tablespace_name_start := INSTR(new_object_file_line, '"', 1, 1) + 1 ;
					tablespace_name_end := INSTR(new_object_file_line, '"', 1, 2);
					dump_tablespace := SUBSTR(new_object_file_line, tablespace_name_start, tablespace_name_end - tablespace_name_start);
					new_object_file_line := REPLACE(new_object_file_line, dump_tablespace, ts);
					-- reset ends_with_tablespace
					ends_with_tablespace := FALSE; -- it is unlikely that this line will end in a TABLESPACE word as well, or it will contain it at all
				END IF;
			END IF;
			
			-- write out the modified line
			UTL_FILE.PUT_LINE(new_object_file, new_object_file_line, TRUE);

		END LOOP;
		UTL_FILE.PUT_LINE(new_object_file, 'SPOOL OFF', TRUE);
		UTL_FILE.PUT_LINE(new_object_file, 'EXIT', TRUE);

		UTL_FILE.FCLOSE(object_file);
		UTL_FILE.FCLOSE(new_object_file);
		
		UTL_FILE.FREMOVE('DATA_PUMP_DIR', object_file_name);
		UTL_FILE.FRENAME('DATA_PUMP_DIR', 'new_' || object_file_name, 'DATA_PUMP_DIR', object_file_name, TRUE);

		-- we keep in mind the schema name generated in the output file, after removing the quotes and the point around it
		-- if it's not the same as our target schema, it is the schema from the dump, and we will need
		-- to use the fromuser parameter with this name in step 4
		dump_schema := REPLACE(dump_schema, '"');
		dump_schema := REPLACE(dump_schema, '.');
		

		-- 3. As the target schema, run the edited <ts>_objects.sql
		jn := SUBSTR('run_' || REPLACE(object_file_name,'.sql',''),1,30);
		DBMS_SCHEDULER.CREATE_JOB(JOB_NAME => jn, JOB_TYPE => 'EXECUTABLE', JOB_ACTION => 'sqlplus', NUMBER_OF_ARGUMENTS => 2); 
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 1, ts || '/' || ts_pwd);
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 2, '@' || dump_dir || '/' || object_file_name);
		DBMS_SCHEDULER.RUN_JOB(JOB_NAME => jn, USE_CURRENT_SESSION => TRUE);  --run job synchronously, this pl/sql block will return only after job has been run
		DBMS_SCHEDULER.DROP_JOB(jn);
		
		-- 4. Run imp with IGNORE=Y (so that it does not error out on object creations. They are already there, created in step 3.)
		-- if dump_schema is the same as our target schema, then run imp ts/ts_pwd file=fn log=jn.log IGNORE=Y full=Y
		-- if dump_schema is not the same as our target schema, then run imp ts/ts_pwd file=fn log=jn.log IGNORE=Y fromuser=dump_schema touser=ts
		--jn := SUBSTR('imp_' || REPLACE(fn, '.','_'),1,30);
		jn := SUBSTR('imp_' || ts,1,30);
		--dbms_output.put_line('job name: ' || jn);
		IF dump_schema = ts THEN
			DBMS_SCHEDULER.CREATE_JOB(JOB_NAME => jn,JOB_TYPE => 'EXECUTABLE',JOB_ACTION =>'?/bin/imp', NUMBER_OF_ARGUMENTS => 5);
		ELSE
			DBMS_SCHEDULER.CREATE_JOB(JOB_NAME => jn,JOB_TYPE => 'EXECUTABLE',JOB_ACTION =>'?/bin/imp', NUMBER_OF_ARGUMENTS => 6);
		END IF;

		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 1, ts || '/' || ts_pwd);
		--DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 2, 'file="' || dump_dir || '/' ||  fn || '"');
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 2, file_clause);
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 3, 'log="' || dump_dir || '/' || jn || '.log' || '"');
		DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 4, 'ignore=Y');
		IF dump_schema = ts THEN
			DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 5, 'full=Y');
		ELSE
			DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 5, 'fromuser=' || dump_schema);
			DBMS_SCHEDULER.SET_JOB_ARGUMENT_VALUE(jn, 6, 'touser=' || ts);
		END IF;

		--DBMS_SCHEDULER.ENABLE(NAME => jn); -run job asynchronously, this pl/sql block will return immediately and the job will run in the background
		DBMS_SCHEDULER.RUN_JOB(JOB_NAME => jn, use_current_session => TRUE);  --run job synchronously, in this session
		DBMS_SCHEDULER.DROP_JOB(jn);
	else
		dbms_output.put_line('Undocumented, dump file type is: '||to_char(ft));
	end if;
EXCEPTION
    WHEN OTHERS THEN
      	err_msg := 'An error has occurred: ' || SUBSTR(SQLERRM, 1, 200);
		dbms_output.put_line(err_msg);
		DBMS_DATAPUMP.DETACH(handle1);
		begin
			DBMS_SCHEDULER.DROP_JOB(jn);
		exception when job_doesnt_exist then
			null;
		end;
		raise_application_error(-20000, err_msg);

END import_dump;
/
SHOW ERRORS

DECLARE
  l_num_errors INTEGER;
BEGIN
  SELECT COUNT(*)
    INTO l_num_errors
    FROM user_errors
   WHERE name = 'IMPORT_DUMP';

 IF( l_num_errors > 0 )
 THEN
   EXECUTE IMMEDIATE 'DROP PROCEDURE IMPORT_DUMP';
   raise_application_error( -20001, 'IMPORT_DUMP stored procedure body could not be compiled, therefore it was dropped! Please correct the errors in sql/oracle/import_procedure.sql and start the agent again.' );
 END IF;
END;
/

EXIT