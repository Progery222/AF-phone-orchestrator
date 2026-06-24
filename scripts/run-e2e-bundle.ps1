# E2E bundle: orchestrator + observer + recovery (+ executor health)
# Run: powershell -ExecutionPolicy Bypass -File scripts/run-e2e-bundle.ps1

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$AF = Split-Path -Parent $Root

$NatsPort = 4222
$OrchHTTP = "19092"
$OrchGRPC = "50055"
$RecoveryDSN = "postgres://recovery:recovery@localhost:5432/recovery?sslmode=disable"

function Stop-PortListener([int]$Port) {
    $matches = netstat -ano | Select-String "LISTENING" | Select-String ":$Port\s"
    foreach ($line in $matches) {
        $procId = ($line.ToString() -split '\s+')[-1]
        if ($procId -match '^\d+$') {
            Write-Host "Stopping PID $procId on port $Port"
            Stop-Process -Id ([int]$procId) -Force -ErrorAction SilentlyContinue
        }
    }
    Start-Sleep -Seconds 1
}

function Wait-Health([string]$Url, [int]$Retries = 20) {
    for ($i = 0; $i -lt $Retries; $i++) {
        try {
            $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
            if ($r.StatusCode -eq 200) { return $true }
        } catch {
            # retry
        }
        Start-Sleep -Seconds 1
    }
    return $false
}

Write-Host "=== 1. NATS dev broker ==="
Stop-PortListener $NatsPort
$natsJob = Start-Job -ScriptBlock {
    param($root, $port)
    Set-Location $root
    $env:NATS_PORT = "$port"
    go run ./cmd/natsdev
} -ArgumentList $Root, $NatsPort
Start-Sleep -Seconds 2

Write-Host "=== 2. Observer (e2e stub serial) ==="
Stop-PortListener 19090
Stop-PortListener 50053
$observerJob = Start-Job -ScriptBlock {
    param($observerRoot)
    Set-Location $observerRoot
    $env:HEALTH_ADDR = "127.0.0.1:19090"
    $env:GRPC_ADDR = ":50053"
    go run ./cmd/server
} -ArgumentList (Join-Path $AF "AF-phone-observer")
Start-Sleep -Seconds 4

Write-Host "=== 3. Recovery-engine ==="
Stop-PortListener 50054
Stop-PortListener 9094
$recoveryJob = Start-Job -ScriptBlock {
    param($recoveryRoot, $natsPort, $dsn)
    Set-Location $recoveryRoot
    $env:GRPC_ADDR = ":50054"
    $env:HEALTH_ADDR = ":9094"
    $env:NATS_URL = "nats://127.0.0.1:$natsPort"
    $env:POSTGRES_DSN = $dsn
    $env:LLM_PROVIDER = "stub"
    go run ./cmd/server
} -ArgumentList (Join-Path $AF "AF-recovery-engine"), $NatsPort, $RecoveryDSN
Start-Sleep -Seconds 5

Write-Host "=== 4. Orchestrator e2e instance ==="
Stop-PortListener $OrchHTTP
Stop-PortListener $OrchGRPC
$orchJob = Start-Job -ScriptBlock {
    param($root, $httpPort, $grpcPort, $natsPort)
    Set-Location $root
    $env:GRPC_ADDR = ":$grpcPort"
    $env:HEALTH_ADDR = ":$httpPort"
    $env:STORE_MODE = "memory"
    $env:OBSERVER_MODE = ""
    $env:RECOVERY_MODE = ""
    $env:OBSERVER_HTTP_URL = "http://127.0.0.1:19090"
    $env:NATS_URL = "nats://127.0.0.1:$natsPort"
    $env:ORCHESTRATOR_TICK_SEC = "1"
    go run ./cmd/server
} -ArgumentList $Root, $OrchHTTP, $OrchGRPC, $NatsPort
Start-Sleep -Seconds 5

Write-Host "=== 5. Wait ready ==="
if (-not (Wait-Health "http://127.0.0.1:19090/health")) {
    Write-Warning "observer :19090 not responding - start AF-phone-observer (make run)"
}
if (-not (Wait-Health "http://127.0.0.1:9094/health")) {
    Receive-Job $recoveryJob -Keep | Select-Object -Last 20
    throw "recovery-engine failed to start"
}
if (-not (Wait-Health "http://127.0.0.1:$OrchHTTP/ready")) {
    Receive-Job $orchJob -Keep | Select-Object -Last 20
    throw "orchestrator e2e not ready"
}
Write-Host "Dependencies ready"

Write-Host "=== 6. go test -tags=e2e ==="
Set-Location $Root
$env:E2E_ORCH_URL = "http://127.0.0.1:$OrchHTTP"
$env:E2E_OBSERVER_URL = "http://127.0.0.1:19090"
$env:E2E_RECOVERY_HEALTH = "http://127.0.0.1:9094"
$env:E2E_EXECUTOR_HEALTH = "http://127.0.0.1:9091"
go test -tags=e2e ./tests/e2e/... -count=1 -v -timeout=5m
$testExit = $LASTEXITCODE

Write-Host "=== 7. Cleanup ==="
Stop-Job $natsJob, $observerJob, $recoveryJob, $orchJob -ErrorAction SilentlyContinue
Remove-Job $natsJob, $observerJob, $recoveryJob, $orchJob -Force -ErrorAction SilentlyContinue

exit $testExit
