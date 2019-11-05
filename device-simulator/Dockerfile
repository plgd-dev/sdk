FROM alpine:3.8 AS build

RUN apk add --no-cache curl git build-base gcc linux-headers patch
RUN git clone --recursive https://github.com/iotivity/iotivity-lite.git
COPY ./patches/devsim.diff /devsim.diff
RUN (cd /iotivity-lite && patch -p1 < /devsim.diff)
RUN make -C /iotivity-lite/port/linux CLOUD=1 SECURE=1 DEBUG=1 MNT=1 cloud_server

FROM alpine:3.8 AS service
COPY --from=build /iotivity-lite/port/linux/cloud_server /usr/local/bin/service
ENTRYPOINT ["/usr/local/bin/service"]