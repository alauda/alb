import errno
import socket
import struct
import threading
import time
from loguru import logger
logger.add("./502.log")

class Http:
    def __str__(self):
        return (
            f"Http Object:\n"
            f"  version_ok = {self.version_ok}\n"
            f"  version = {self.version}\n"
            f"  method = {self.method}\n"
            f"  url_ok = {self.url_ok}\n"
            f"  url = {self.url}\n"
            f"  header_ok = {self.header_ok}\n"
            f"  header = {self.header}\n"
            f"  body_ok = {self.body_ok}\n"
            f"  all_ok = {self.all_ok}\n"
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
        self.all_ok = False
        self.body_ok = False
        self.body_start = False
        self.method=None
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
        if self.method=="GET" and self.header_ok:
            self.all_ok=True
            return
        # Parsing the body
        if len(parts) > 1:
            self.body_start=True
            self.body = parts[1].encode()
            if 'Content-Length' in self.header:
                self.body_ok = int(self.header['Content-Length']) == len(self.body)
                self.all_ok=self.body_ok


class BreakServer:
    def __init__(self) -> None:
        self.all_bytes=b''
        
    def break_conn(self,conn:socket):
        linger_struct = struct.pack('ii', 1, 0)
        conn.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, linger_struct)
        conn.close()
    def on_recv(self,conn:socket,data:bytes):
        self.all_bytes+=data
        if self.http.body_ok:
            body="ok"
            conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
            conn.close()
            return "break"
        if self.http.all_ok and self.http.method=="GET" and len(data)==0:
            body="ok"
            conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
            logger.debug("just close")
            conn.close()
            return "break"
def try_read(conn:socket,size):
    try:
        msg = conn.recv(size)
        return msg,False,None
    except socket.error as e:
        err = e.args[0]
        if err == errno.EAGAIN or err == errno.EWOULDBLOCK:
            return bytes(),True,None
        else:
            return bytes(),False,str(e)
    pass
def server(host,port,hs):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        logger.debug(f"start {host} {port}")
        s.bind((host, port))
        s.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 100)
        s.listen()
        while True:
            conn, addr = s.accept()
            h=hs()
            with conn:
                conn.setblocking(False)
                logger.debug(f"Connected by {addr}")
                chunk=bytes()
                while True:
                    data,over,err = try_read(conn,1024)
                    chunk=chunk+data
                    if not over:
                        continue
                    if err!=None:
                        logger.debug(err)
                        break
                    http=Http(chunk)
                    logger.debug("{} {}",str(http),str(chunk))
                    if http.all_ok:
                        body="ok"
                        if "11_n" in http.url:
                            conn.sendall( f"HTTP/1.1 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
                        elif "10_n" in http.url:
                            conn.sendall( f"HTTP/1.0 200 OK\nContent-Length: {len(body)}\n\n{body}".encode())
                        elif "11_c" in http.url:
                            conn.sendall( f"HTTP/1.1 200 OK\nConnection: close\nContent-Length: {len(body)}\n\n{body}".encode())
                        elif "10_c" in http.url:
                            conn.sendall( f"HTTP/1.0 200 OK\nConnection: close\nContent-Length: {len(body)}\n\n{body}".encode())
                            pass
                        logger.debug("just close")
                        conn.close()
                        break
                    pass
                pass
            pass
        pass
    pass
pass


HOST = "127.0.0.1"
t1 = threading.Thread(target=server, args=(HOST, 65432,lambda :BreakServer()))
t1.start()
