#!/usr/bin/env python3
"""
Simple SOCKS5 proxy server for StreamArr tunnel
Runs on Mac, forwards traffic through home IP
"""
import socket
import select
import struct
import sys

SOCKS_VERSION = 5

class SOCKSProxy:
    def __init__(self, host='127.0.0.1', port=1080):
        self.host = host
        self.port = port
        
    def handle_client(self, client_sock):
        # SOCKS5 greeting
        version, nmethods = struct.unpack("!BB", client_sock.recv(2))
        methods = client_sock.recv(nmethods)
        client_sock.sendall(struct.pack("!BB", SOCKS_VERSION, 0))  # No auth
        
        # SOCKS5 request
        version, cmd, _, address_type = struct.unpack("!BBBB", client_sock.recv(4))
        
        if address_type == 1:  # IPv4
            address = socket.inet_ntoa(client_sock.recv(4))
        elif address_type == 3:  # Domain name
            domain_length = client_sock.recv(1)[0]
            address = client_sock.recv(domain_length).decode('utf-8')
        else:
            client_sock.close()
            return
            
        port = struct.unpack('!H', client_sock.recv(2))[0]
        
        try:
            if cmd == 1:  # CONNECT
                remote = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                remote.connect((address, port))
                bind_address = remote.getsockname()
                addr = struct.unpack("!I", socket.inet_aton(bind_address[0]))[0]
                port = bind_address[1]
                
                reply = struct.pack("!BBBBIH", SOCKS_VERSION, 0, 0, 1, addr, port)
            else:
                reply = struct.pack("!BBBBIH", SOCKS_VERSION, 7, 0, 1, 0, 0)
                
            client_sock.sendall(reply)
            
            if cmd == 1:
                self.forward(client_sock, remote)
        except Exception as e:
            print(f"Error: {e}")
        finally:
            client_sock.close()
            
    def forward(self, client, remote):
        while True:
            r, w, e = select.select([client, remote], [], [], 3)
            if client in r:
                data = client.recv(4096)
                if remote.send(data) <= 0:
                    break
            if remote in r:
                data = remote.recv(4096)
                if client.send(data) <= 0:
                    break
                    
    def run(self):
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind((self.host, self.port))
        s.listen(10)
        print(f"🔒 SOCKS5 proxy running on {self.host}:{self.port}")
        
        while True:
            try:
                client, addr = s.accept()
                import threading
                threading.Thread(target=self.handle_client, args=(client,), daemon=True).start()
            except KeyboardInterrupt:
                break
        s.close()

if __name__ == '__main__':
    proxy = SOCKSProxy()
    proxy.run()
