FROM debian:stretch AS release

# Install sudo because entrypoint.sh uses it
RUN apt-get -y update && \
    apt-get install -y --no-install-recommends sudo ca-certificates && \
    apt-get autoremove && apt-get clean && \
    apt-get rclone && unzip

RUN mkdir -p /home/.config/rclone
COPY ./rclone/rclone.conf /home/.config/rclone

RUN curl https://storage.googleapis.com/bysh-chef-files/olaris-release/olaris-linux-amd64-v0.3.3.zip
RUN unzip olaris-linux-amd64-v0.3.3.zip


VOLUME /home/.config/
EXPOSE 8080
ENTRYPOINT ["/bin/olaris"]
