proxy_http_version 1.1;
# Empty Connection header lets nginx reuse upstream keepalive connections.
proxy_set_header Connection "";
proxy_set_header Host $host;
# Overwrite (not append) so clients cannot spoof their IP for rate limiting.
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_read_timeout 30s;
