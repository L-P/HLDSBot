FROM cm2network/steamcmd:steam-bookworm

# Running the install twice because the first execution will always fail with
# code 0x10E.
RUN set -eux; \
    ./steamcmd.sh +force_install_dir /home/steam/hlds +login anonymous +app_update 90 +quit || \
    ./steamcmd.sh +force_install_dir /home/steam/hlds +login anonymous +app_update 90 +quit;

USER root
COPY hlds.entrypoint /usr/bin/hlds.entrypoint
RUN chmod +x /usr/bin/hlds.entrypoint

USER steam
WORKDIR /home/steam/hlds
COPY server.cfg instance.cfg listip.cfg banned.cfg /home/steam/hlds/valve/

ENTRYPOINT ["/usr/bin/hlds.entrypoint"]
