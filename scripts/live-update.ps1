param(
    [string]$ApiBase = 'http://localhost:8080',
    [string]$AdminKey = 'admin-123',
    [string]$FlagKey = 'checkout_new_ui'
)

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Url,
        [hashtable]$Headers,
        [string]$Body
    )

    try {
        return Invoke-WebRequest -Method $Method -Uri $Url -Headers $Headers -Body $Body -ContentType 'application/json'
    }
    catch {
        Write-Error "Request to $Url failed: $($_.Exception.Message)"
        if ($_.Exception.Response -and $_.Exception.Response.Content) {
            Write-Error ([System.Text.Encoding]::UTF8.GetString($_.Exception.Response.Content))
        }
        throw
    }
}

Write-Host "Fetching current snapshot from $ApiBase..."
$snapshot = Invoke-Json -Method 'GET' -Url "$ApiBase/v1/flags/snapshot?ts=$(Get-Random)" -Headers @{}
$oldEtag = $snapshot.Headers.ETag
Write-Host "Current ETag: $oldEtag"

$timestamp = [DateTime]::UtcNow.ToString('o')
$payload = @{
    key = $FlagKey
    description = "updated $timestamp"
    enabled = $true
    rollout = 50
    config = @{ variant = $timestamp }
    env = 'prod'
} | ConvertTo-Json -Depth 4

Write-Host "Posting flag update..."
$headers = @{ Authorization = "Bearer $AdminKey" }
Invoke-Json -Method 'POST' -Url "$ApiBase/v1/flags" -Headers $headers -Body $payload | Out-Null

Write-Host "Re-fetching snapshot with If-None-Match: $oldEtag"
$newHeaders = @{ 'If-None-Match' = $oldEtag }
$newSnapshot = Invoke-Json -Method 'GET' -Url "$ApiBase/v1/flags/snapshot?ts=$(Get-Random)" -Headers $newHeaders
$newEtag = $newSnapshot.Headers.ETag
Write-Host "New ETag: $newEtag"

if ($newEtag -eq $oldEtag) {
    Write-Warning 'ETag did not change; snapshot may be stale.'
} else {
    Write-Host 'Snapshot updated successfully.'
}

$newSnapshot.Content | Write-Output
