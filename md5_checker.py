import os
import hashlib

def calculate_md5(file_path):
    """计算文件的MD5哈希值"""
    hash_md5 = hashlib.md5()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(4096), b""):
            hash_md5.update(chunk)
    return hash_md5.hexdigest()

def main():
    # 目录路径
    send_files_dir = "flute_sender/send_files"
    received_files_dir = "flute_receiver/received_files"

    print("MD5 hashes for files in send_files (sender):")
    if os.path.exists(send_files_dir):
        for root, dirs, files in os.walk(send_files_dir):
            for file in files:
                file_path = os.path.join(root, file)
                md5_hash = calculate_md5(file_path)
                print(f"{file_path}: {md5_hash}")
    else:
        print(f"Directory {send_files_dir} does not exist.")

    print("\nMD5 hashes for files in received_files:")
    if os.path.exists(received_files_dir):
        for root, dirs, files in os.walk(received_files_dir):
            for file in files:
                file_path = os.path.join(root, file)
                md5_hash = calculate_md5(file_path)
                print(f"{file_path}: {md5_hash}")
    else:
        print(f"Directory {received_files_dir} does not exist.")

if __name__ == "__main__":
    main()