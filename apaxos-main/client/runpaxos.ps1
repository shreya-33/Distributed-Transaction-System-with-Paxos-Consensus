$servers = @(
    @{Name="S1"; Port="50051"},
    @{Name="S2"; Port="50052"},
    @{Name="S3"; Port="50053"},
    @{Name="S4"; Port="50054"},
    @{Name="S5"; Port="50055"}
)

$wtPath = "wt"
$scriptPath = "C:\Users\akothuru\Desktop\apaxos-kothuruabhilashreddy\paxos.go"
$projectPath = "C:\Users\akothuru\Desktop\apaxos-kothuruabhilashreddy"

# Open the first server
$firstServer = $servers[0]
$firstCommand = "cd `"$projectPath`"; go run `"$scriptPath`" $($firstServer.Name)"
Start-Process $wtPath -ArgumentList "new-tab", "--title", $firstServer.Name, "commandprompt", "-NoExit", "-Command", $firstCommand

# Open subsequent servers
for ($i = 1; $i -lt $servers.Length; $i++) {
    $server = $servers[$i]
    $command = "cd `"$projectPath`"; go run `"$scriptPath`" $($server.Name)"
    Start-Process $wtPath -ArgumentList "new-tab", "--title", $server.Name, "commandprompt", "-NoExit", "-Command", $command
}
