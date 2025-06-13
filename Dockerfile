FROM alpine:latest

ARG TARGETARCH

ENV TZ=Asia/Shanghai

RUN apk --no-cache add ca-certificates \
	tzdata

WORKDIR /app

COPY ./build/linux_${TARGETARCH} /app/

LABEL Name=GoDDD Version=0.0.1

EXPOSE 8080

CMD [ "./bin" ]