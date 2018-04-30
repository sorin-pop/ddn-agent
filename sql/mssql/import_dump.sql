SET NOCOUNT ON;

DECLARE @fileListTable TABLE (
    [LogicalName]           NVARCHAR(128),
    [PhysicalName]          NVARCHAR(260),
    [Type]                  CHAR(1),
    [FileGroupName]         NVARCHAR(128),
    [Size]                  NUMERIC(20,0),
    [MaxSize]               NUMERIC(20,0),
    [FileID]                BIGINT,
    [CreateLSN]             NUMERIC(25,0),
    [DropLSN]               NUMERIC(25,0),
    [UniqueID]              UNIQUEIDENTIFIER,
    [ReadOnlyLSN]           NUMERIC(25,0),
    [ReadWriteLSN]          NUMERIC(25,0),
    [BackupSizeInBytes]     BIGINT,
    [SourceBlockSize]       INT,
    [FileGroupID]           INT,
    [LogGroupGUID]          UNIQUEIDENTIFIER,
    [DifferentialBaseLSN]   NUMERIC(25,0),
    [DifferentialBaseGUID]  UNIQUEIDENTIFIER,
    [IsReadOnly]            BIT,
    [IsPresent]             BIT,
    [TDEThumbprint]         VARBINARY(32) -- necessary, starting with SQL Server 2008
	--[SnapshotURL]			NVARCHAR(360) -- necessary, starting with SQL Server ?
)
DECLARE @RestoreStatement NVARCHAR(MAX), 
        @dumpFileEntryType CHAR(1), 
        @dumFileEntryLogicalName NVARCHAR(128),
        @localDataFolder NVARCHAR(MAX),
		@dumpFile NVARCHAR(MAX) = '$(driveLetter)' + ':' + '$(dumpPath)',
		@newPhysicalName NVARCHAR(128),
		@fileType CHAR(1)

SELECT top(1) @localDataFolder =  physical_name FROM sys.master_files;  
SET  @localDataFolder = REPLACE(@localDataFolder, RIGHT(@localDataFolder, CHARINDEX('\', REVERSE(@localDataFolder))-1),'');
--print @localDataFolder      

INSERT INTO @fileListTable EXEC('RESTORE FILELISTONLY FROM DISK = N''' + @dumpFile + '''')

--SELECT * FROM @fileListTable

SET @RestoreStatement = N'RESTORE DATABASE [' + '$(targetDatabaseName)]' + N' FROM DISK=N''' + @dumpFile + '''' + N' WITH REPLACE, '

DECLARE dumpFileList CURSOR FOR
	SELECT
		Type,
		LogicalName
		--LTRIM(RTRIM(RIGHT(PhysicalName, CHARINDEX('\', REVERSE(PhysicalName)) - 1)))
	FROM @fileListTable WHERE Type IN ('D','L');

OPEN dumpFileList 
    FETCH NEXT FROM dumpFileList INTO @fileType, @dumFileEntryLogicalName;
    WHILE @@Fetch_Status = 0
    BEGIN
		IF @fileType = 'D' 
			SET @newPhysicalName = '$(targetDatabaseName)' + '.mdf';
		IF @fileType = 'L' 
			SET @newPhysicalName = '$(targetDatabaseName)' + '_log.ldf';
		SET @RestoreStatement = @RestoreStatement + 'MOVE ' + '''' + @dumFileEntryLogicalName + '''' + 
        ' TO ' + '''' + @localDataFolder +  @newPhysicalName  + '''' + ', ';
		FETCH NEXT FROM dumpFileList INTO @fileType, @dumFileEntryLogicalName;
    END

CLOSE dumpFileList
DEALLOCATE dumpFileList

set @RestoreStatement = substring(@RestoreStatement, 1, len(@RestoreStatement)-1);

--PRINT @RestoreStatement

EXEC(@RestoreStatement)

