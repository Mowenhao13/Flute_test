import platform
import subprocess
import re

def get_network_interfaces():
    system = platform.system()
    
    if system == "Windows":
        # Windows
        result = subprocess.run(['ipconfig', '/all'], capture_output=True, text=True)
        interfaces = re.findall(r'([\w\s]+适配器 [\w\s]+):.*?物理地址[\. ]+ : ([\w-]+)', result.stdout, re.DOTALL)
        return [(name.strip(), mac) for name, mac in interfaces]
    
    elif system == "Darwin":  # macOS
        result = subprocess.run(['ifconfig'], capture_output=True, text=True)
        interfaces = re.findall(r'^(\w+):.*?ether ([\w:]+)', result.stdout, re.MULTILINE | re.DOTALL)
        return interfaces
    
    elif system == "Linux":
        result = subprocess.run(['ip', 'link', 'show'], capture_output=True, text=True)
        interfaces = re.findall(r'^\d+: (\w+):.*?link/ether ([\w:]+)', result.stdout, re.MULTILINE)
        return interfaces
    
    else:
        return []

# 使用示例
if __name__ == "__main__":
    interfaces = get_network_interfaces()
    for name, mac in interfaces:
        print(f"接口: {name}, MAC地址: {mac}")