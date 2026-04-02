Write-Host "Update Shared" -ForegroundColor Green
cd ..\trip-builder\shared
go mod tidy

Write-Host "Update Auth" -ForegroundColor Green
cd ..\services\auth
go mod tidy
go mod vendor

Write-Host "Update User" -ForegroundColor Green
cd ..\user
go mod tidy
go mod vendor

Write-Host "Update Gateway" -ForegroundColor Green
cd ..\gateway
go mod tidy
go mod vendor

Write-Host "Update Route" -ForegroundColor Green
cd ..\route
go mod tidy
go mod vendor

Write-Host "Update Event" -ForegroundColor Green
cd ..\event
go mod tidy
go mod vendor

Write-Host "Update Task" -ForegroundColor Green
cd ..\task
go mod tidy
go mod vendor

Write-Host "Back road" -ForegroundColor Green
cd ..\..\..\.scripts