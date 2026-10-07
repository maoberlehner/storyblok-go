# The official nginx image lacks the brotli module; Alpine packages it.
FROM alpine:3.23
RUN apk add --no-cache nginx nginx-mod-http-brotli \
    && ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log \
    && install -d -o nginx -g nginx /var/cache/nginx /run/nginx
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 443
STOPSIGNAL SIGQUIT
CMD ["nginx", "-g", "daemon off;"]
