# Performance

## Test Configuration

```
go run main.go StandAlone
```

Router and WriteNode are on Localhost with nominal load.

## Baseline `e374012`

```
Measure-Command {
    python .\send_to_tcp_port.py file 5001 56k.jpg
}

TotalMilliseconds : 6608.5445
TotalMilliseconds : 6435.1571
TotalMilliseconds : 4067.9861
TotalMilliseconds : 4168.8002
TotalMilliseconds : 4021.2115
```
Min: 4021.2115
Mean: 5060.33988
Max: 6608.5445


## Switch from fmt to log

```
TotalMilliseconds : 7359.0541
TotalMilliseconds : 4167.4938
TotalMilliseconds : 4461.7202
TotalMilliseconds : 3949.8589
TotalMilliseconds : 4157.0019
```

Min: 3949.8589
Mean: 4819.02578
Max: 7359.0541

## Analysis

While cache effects are difficult to rule out, switching to `log` from `fmt` appears to give a 5% gain on average.