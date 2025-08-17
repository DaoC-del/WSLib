Param(
  [ValidateSet("up","down","restart","logs")]
  [string]$Cmd = "up"
)
switch ($Cmd) {
  "up"      { docker compose up -d }
  "down"    { docker compose down }
  "restart" { docker compose up -d --build }
  "logs"    { docker compose logs -f }
}
