IF NOT EXISTS (SELECT name FROM [sys].[server_principals] WHERE name = '$(name)')
Begin
    CREATE LOGIN $(name) WITH PASSWORD = '$(password)';
    CREATE USER $(name) FOR LOGIN $(name);
    GRANT ALL PRIVILEGES TO $(name);
    ALTER SERVER ROLE [dbcreator] ADD MEMBER [$(name)];
End
GO