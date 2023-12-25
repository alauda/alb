import socket
import struct
import threading
import time

def server(host,port,hs):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        print(f"start {host} {port}")
        s.bind((host, port))
        s.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 100)
        s.listen()
        while True:
            conn, addr = s.accept()
            h=hs()
            with conn:
                print(f"Connected by {addr}")
                while True:
                    data = conn.recv(10)
                    cmd = h.on_recv(conn,data)
                    if cmd=="break":
                        break
class Http:
    def __str__(self):
        return (
            f"Http Object:\n"
            f"  version_ok = {self.version_ok}\n"
            f"  version = {self.version}\n"
            f"  url_ok = {self.url_ok}\n"
            f"  url = {self.url}\n"
            f"  header_ok = {self.header_ok}\n"
            f"  header = {self.header}\n"
            f"  body_ok = {self.body_ok}\n"
            f"  body = {self.body.decode('utf-8', 'replace') if self.body else None}\n"
        )
    def __init__(self, data: bytes) -> None:
        self.version_ok = False
        self.version = ''
        self.url_ok = False
        self.url = ''
        self.header_ok = False
        self.header_start = False
        self.header = {}
        self.body_ok = False
        self.body_start = False
        self.body = b''

        data_str = data.decode()
        parts = data_str.split('\r\n\r\n', 1)

        # Parsing the headers
        headers = parts[0].split('\r\n')
        request_line = headers.pop(0).split()

        # Parsing the request line
        if len(request_line) == 3:
            self.method = request_line[0]
            self.url = request_line[1]
            self.version = request_line[2]
            self.url_ok = True
            self.version_ok = self.version in ['HTTP/1.0', 'HTTP/1.1', 'HTTP/2.0']

        # Parsing the headers
        if headers:
            self.header_start=True
            for header in headers:
                if ':' in header:
                    key, value = map(str.strip, header.split(':', 1))
                    self.header[key] = value
            self.header_ok = True

        # Parsing the body
        if len(parts) > 1:
            self.body_start=True
            self.body = parts[1].encode()
            if 'Content-Length' in self.header:
                self.body_ok = int(self.header['Content-Length']) == len(self.body)
class NormalServer:
    def __init__(self) -> None:
        self.all_bytes=b''
    def on_recv(self,conn:socket,data:bytes):
        self.all_bytes+=data
        self.http=Http(self.all_bytes)
        # print(f"normal server {data}  {len(self.http.body)}")
        if self.http.body_ok:
            print("end")
            body="ok"
            conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
            conn.close()
            return "break"


class BreakServer:
    def __init__(self) -> None:
        self.all_bytes=b''
        
    def break_conn(self,conn:socket):
        linger_struct = struct.pack('ii', 1, 0)
        conn.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, linger_struct)
        conn.close()
        
    def on_break_on_header(self,conn:socket):
        if self.http.header_start and len(self.http.header)!=0 :
            print(f"i get some header {self.http.header}, i break")
            self.break_conn(conn)
            return "break"
            
        
    def on_break_on_res(self,conn:socket,t):
        if self.http.body_ok:
            if t=="timeout-header":
                time.sleep(30)
                body="after timeuot"
                conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
                return "break"
                
            if t=="timeout-body":
                body="after timeuot"
                conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n".encode())
                time.sleep(30)
                conn.senda( f"{body}".encode())
                return "break"
                
            if t=="header-middle":
                conn.sendall( f"HTTP/1.0 200 OK\na".encode())
                self.break_conn(conn)
                return "break"
                
            if t=="start":
                self.break_conn(conn)
                return "break"
                
            body="half-all"
            conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n".encode())
            self.break_conn(conn)
            return "break"
            
    def on_break_on_body(self,conn:socket):
        if self.http.body_start and len(self.http.body)!=0 :
            print(f"i get some body {self.http.body}, i break")
            self.break_conn(conn)
            return "break"
            

        
    def on_recv(self,conn:socket,data:bytes):
        if len(data)==0:
            print("end")
            return "break"
            
        self.all_bytes+=data
        self.http=Http(self.all_bytes)
        print(self.http)
        if self.http.url_ok:
            if "break-on-header" in self.http.url:
                return self.on_break_on_header(conn)
                
            if "break-on-body" in self.http.url:
                return self.on_break_on_body(conn)
                
            if "break-on-res-start" in self.http.url:
                return self.on_break_on_res(conn,"start")
                
            if "break-on-res-middle" in self.http.url:
                return self.on_break_on_res(conn,"middle")
                
            if "break-on-res-header-middle" in self.http.url:
                return self.on_break_on_res(conn,"header-middle")
                
            if "break-on-res-timeout-header" in self.http.url:
                return self.on_break_on_res(conn,"timeout-header")
                
            if "break-on-res-timeout-body" in self.http.url:
                return self.on_break_on_res(conn,"timeout-body")
                
            
        
    

HOST = "127.0.0.1"
t1 = threading.Thread(target=server, args=(HOST, 65432,lambda :BreakServer()))
t1.start()

# 创建并启动第二个线程
t2 = threading.Thread(target=server, args=(HOST, 65433,lambda:NormalServer()))
t2.start()

