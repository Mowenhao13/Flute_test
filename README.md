# FluteTest
## 项目结构
```txt
FLUTE_TEST/
├── cmd/
│   ├── received_files/          # 接收文件目录
│   ├── send_files/              # 发送文件目录
│   ├── flute_receiver/          # 接收器执行文件
│   └── flute_sender/            # 发送器执行文件
├── config/
│   ├── receiverCfg.yaml         # 接收器配置文件
│   └── senderCfg.yaml          # 发送器配置文件
├── flute_receiver/
│   └── receiver.go              # 接收器主程序
├── flute_sender/
│   └── sender.go                # 发送器主程序
├── pkg/                         # 核心包目录
│   ├── alc/
│   │   └── alc.go               # ALC协议实现
│   ├── encoder/
│   │   └── encoder.go           # 编码器测试
│   ├── fdt/
│   │   └── fdt.go               # 文件描述表(FDT)实现
│   ├── filedesc/
│   │   └── filedesc.go          # 文件描述符实现
│   ├── lct/
│   │   └── lct.go               # LCT协议实现
│   ├── oti/
│   │   └── oti.go               # 对象传输信息(OTI)实现
│   ├── sender/
│   │   └── sender.go            # 发送器核心逻辑
│   ├── udpendpoint/
│   │   └── endpoint.go          # UDP端点实现
│   └── utils/
│       └── utils.go             # 工具函数
├── go.mod                       # Go模块定义
├── go.sum                      # 依赖校验和
├── md5_checker.py              # MD5校验脚本
└── check_device.py             # 自动获取MAC地址和网络接口
```

## 技术方案
采用 udp 单播，通过采用 fec 前向纠错方案加静态arp配置实现无连接单向传输

## 前置配置
1. 需要获取发送端和接收端双方的 MAC 地址, IPv4 地址以及设备网络接口名称，设置相同的端口
2. 在配置文件里按照发送顺序设置收发文件路径（文件的 `content_type` 可忽略）
3. 收发文件路径是相对 `flute_sender/sender.go` 和 `flute_receiver/receiver.go`，也可以写成绝对路径，要注意不同系统之间文件路径格式的差异
4. 默认关闭静态arp，需在配置文件里将 `static_arp/enable` 设置成 `true` 
5. 若出现 `failed to calc params`的错误，可将 `config/senderCfg.yaml`里的 `fec/encoding_symbol_length` 调大或调小，但最大不超过 `65535`
6. 为了防止文件传输失败，可以调整内核设置，这里给出 linux 系统下的内核调整参考

## Linux 内核参数调整参考
均为临时设置，重启后失效
```zsh
# 调整UDP缓冲区大小
sudo sysctl -w net.core.rmem_max=134217728  # 设置最大接收缓冲区为 128 MB
sudo sysctl -w net.core.rmem_default=134217728  # 设置默认接收缓冲区为 128 MB
sudo sysctl -w net.core.wmem_max=134217728
sudo sysctl -w net.core.netdev_max_backlog=65535

# 设置MTU
sudo ip link set dev veth0 mtu 65535
sudo ip link set dev veth1 mtu 65535

# 增大发送队列长度
sudo ip link set dev veth0 txqueuelen 65535
sudo ip link set dev veth1 txqueuelen 65535
```

## 默认配置文件
### 接收端
- `peer_ip`: 接收端 IP 
- `peer_mac`: 发送端端 MAC 地址
- `interface`: 发送端网络接口
- `listen_ip`: 接收端 IP
```yaml
# config/receiverCfg.yaml
static_arp:
  enable: false
  interface: eth0
  peer_ip: 192.168.1.103
  peer_mac: "00:11:22:33:44:55"

network:
  listen_ip: 192.168.1.102
  port: 3400

storage:
  save_dir: ./cmd/received_files
```
### 发送端
- `peer_ip`: 接收端 IP 
- `peer_mac`: 接收端 MAC 地址
- `interface`: 接收端网络接口
- `source_ip`: 发送端 IP 
- `dest_ip`: 接收端IP地址
- `port`: 端口(注意不要被其他程序占用)
- `fec/type`: 是否启用 fec 编码，`no-code`表示不启用，`RaptorQ`表示启用 `RaptorQ`方案
```yaml
# config/senderCfg.yaml
static_arp:
  enable: false
  interface: eth0
  peer_ip: 192.168.1.103
  peer_mac: "00:11:22:33:44:55"

network:
  source_ip: 192.168.1.102 
  dest_ip: 192.168.1.103
  port: 3400
  
transmission:
  fdt_duration_ms: 1000
  fdt_start_id: 1

files:
  - path: ./cmd/send_files/test_1mb.bin
  name: test_1mb.bin
  content_type: application/octet-stream
  - path: ./cmd/send_files/test_100mb.bin
  name: test_100mb.bin
  content_type: application/octet-stream

fec:
  type: no-code
  encoding_symbol_length: 10240
```
## 启动
先启动接收端再启动发送端
###  接收端（终端1）
```zsh
cd /path/to/FluteTest/
./cmd/flute_receiver
```
### 发送端（终端2）
```zsh
cd /path/to/FluteTest/
./cmd/flute_sender
```
