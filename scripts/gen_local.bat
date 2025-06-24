@echo off
if "%CURSEFORGE_API_KEY%"=="" (
    echo Please set the CURSEFORGE_API_KEY environment variable.
    exit /b 1
)
if "%DISCORD_CLIENT_ID%"=="" (
    echo Please set the DISCORD_CLIENT_ID environment variable.
    exit /b 1
)
if "%MSA_CLIENT_ID%"=="" (
    echo Please set the MSA_CLIENT_ID environment variable.
    exit /b 1
)
echo {
echo   "CURSEFORGE_API_KEY": "%CURSEFORGE_API_KEY%",
echo   "DISCORD_CLIENT_ID": "%DISCORD_CLIENT_ID%",
echo   "MSA_CLIENT_ID": "%MSA_CLIENT_ID%"
echo }
