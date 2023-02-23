package configmap

const HTTP = `
	log_format  http  '[$time_local] $remote_addr "$host" "$request" '
                      '$status $upstream_status $upstream_addr '
                      '"$http_user_agent" "$http_x_forwarded_for" '
                      '$request_time $upstream_response_time';
    access_log  /dev/stdout  http buffer=16k flush=1s;
    error_log   stderr       info;
    rewrite_log on;

    # Show alb info to identify error response
    server_tokens off;
    more_set_headers 'Via: alb'
    proxy_pass_header Server;

    # Disable checking of client request body size
    client_max_body_size 0;

    # Lua shared dict
    lua_code_cache on;
    lua_package_path '/usr/local/lib/lua/?.lua;/alb/nginx/lua/?.lua;/alb/nginx/lua/vendor/?.lua;;';
    lua_package_cpath '/usr/local/lib/lua/?.so;;';
    lua_shared_dict http_policy   10m;
    lua_shared_dict http_certs_cache   10m;
    lua_shared_dict http_backend_cache 5m;
    lua_shared_dict http_alb_cache     20m;
    lua_shared_dict http_raw     5m;
    lua_shared_dict prometheus_metrics 10m;
    lua_shared_dict http_ipc_shared_dict 1m;

    proxy_connect_timeout      5s;
    proxy_send_timeout         120s;
    proxy_read_timeout         120s;
    proxy_buffering            off;
    proxy_buffer_size          4k;
    proxy_buffers              4 32k;
    proxy_busy_buffers_size    64k;
    proxy_temp_file_write_size 64k;
    # keepalive to improve http performance
    keepalive_timeout  65;
    proxy_next_upstream_tries   5;

    sendfile        off;
    sendfile_max_chunk 2147483647;
    aio             threads;
    tcp_nopush      on;
    tcp_nodelay     on;
    log_subrequest  on;
    reset_timedout_connection on;

    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      '';
    }

    # Start error if set lower
    server_names_hash_bucket_size 512;

    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 10m;
    ssl_prefer_server_ciphers on;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2 TLSv1.3;
    ssl_early_data off;
    ssl_ciphers TLS13-AES-256-GCM-SHA384:TLS13-CHACHA20-POLY1305-SHA256:TLS13-AES-128-GCM-SHA256:TLS13-AES-128-CCM-8-SHA256:TLS13-AES-128-CCM-SHA256:EECDH+ECDSA+AESGCM:EECDH+aRSA+AESGCM:EECDH+ECDSA+SHA512:EECDH+ECDSA+SHA384:EECDH+ECDSA+SHA256:ECDH+AESGCM:DH+AESGCM:RSA+AESGCM:!aNULL:!eNULL:!LOW:!RC4:!3DES:!MD5:!EXP:!PSK:!SRP:!DSS;
    ssl_dhparam /etc/alb2/nginx/dhparam.pem;
    ssl_ecdh_curve secp384r1;

    large_client_header_buffers            4 16k;
    keepalive_requests              1000;
    http2_max_concurrent_streams    128;
`

const HTTPSERVER = `
    # fix http://jira.alaudatech.com/browse/DEV-15515, use lua instead
    set              $custom_host      $http_host;
    proxy_set_header Host              $custom_host;
    proxy_set_header Upgrade           $http_upgrade;
    proxy_set_header Connection        $connection_upgrade;

    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    # fix http://jira.alauda.cn/browse/DEVOPS-5309
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host  $http_host;
    proxy_set_header X-Forwarded-Port  $server_port;

    proxy_set_header X-Original-URI $request_uri;
    proxy_set_header X-Scheme $scheme;
    proxy_set_header X-Original-URL $scheme://$http_host$request_uri;
    proxy_set_header X-Original-Method $request_method;

    proxy_redirect   off;
    proxy_http_version 1.1;

    # fix http://jira.alauda.cn/browse/SGHL-142
    underscores_in_headers on;
`

const GRPCSERVER = `
    grpc_set_header   Content-Type application/grpc;
    grpc_socket_keepalive on;
    keepalive_requests 1000;
`

const STREAM_COMMON = `
    log_format stream '[$time_local] $remote_addr $protocol $server_port $status $bytes_received $bytes_sent $session_time';

    access_log  /dev/stdout  stream;
    error_log   stderr       info;

    lua_code_cache on;
    lua_package_path '/usr/local/lib/lua/?.lua;/alb/nginx/lua/?.lua;/alb/nginx/lua/vendor/?.lua;;';
    lua_package_cpath '/usr/local/lib/lua/?.so;;';
    
    # Lua shared dict
    lua_shared_dict stream_policy   10m;
    lua_shared_dict stream_backend_cache 5m;
    lua_shared_dict stream_alb_cache     20m;
    lua_shared_dict stream_raw     5m;
    lua_shared_dict stream_ipc_shared_dict 1m;
`

const STREAM_TCP = `
    proxy_next_upstream_tries 5;
    proxy_connect_timeout      5s;
    proxy_timeout              120s;
    tcp_nodelay                on;
`
const STREAM_UDP = `
    proxy_next_upstream_tries 5;
    proxy_connect_timeout      5s;
    proxy_timeout              120s;
`

const UPSTREAM = `
    keepalive 320;
`
