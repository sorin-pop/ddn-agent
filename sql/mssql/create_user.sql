IF NOT EXISTS (SELECT name FROM [sys].[server_principals] WHERE name = 'clouddb')
Begin
    CREATE LOGIN clouddb WITH PASSWORD = 'Password1234';
    CREATE USER clouddb FOR LOGIN clouddb;
    GRANT ALL PRIVILEGES TO clouddb;
    ALTER SERVER ROLE [dbcreator] ADD MEMBER [clouddb];
end
GO
