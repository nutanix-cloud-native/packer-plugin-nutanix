$IP = "10.2.3.4"
$MaskBits = 24
$Gateway = "10.2.3.1"
$DNS = "1.1.1.1"
$IPType = "IPv4"

$Adapter = Get-NetAdapter | Where-Object { $_.Status -eq "Up" }

if (($Adapter | Get-NetIPConfiguration).Ipv4Address.IpAddress) {
    $Adapter | Remove-NetIPAddress -AddressFamily $IPType -Confirm:$false
}

if (($Adapter | Get-NetIPConfiguration).Ipv4DefaultGateway){
    $Adapter | Remove-NetRoute -AddressFamily $IPType -Confirm:$false
}

$Adapter | New-NetIPAddress -IPAddress $IP -PrefixLength $MaskBits -DefaultGateway $Gateway -AddressFamily $IPType

$Adapter | Set-DnsClientServerAddress -ServerAddresses $DNS -AddressFamily $IPType