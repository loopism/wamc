# WAMC

Based strongly on https://github.com/420m/dockyard, but intended for running on a home server.
Inspiration taken from https://github.com/zackbcom/media-server and https://github.com/mandreko/media-server

## First run

- install [Docker](https://www.docker.com/)
- create a [Plex accout](https://www.plex.tv/)
- clone this repository
- create a user for your media server.
- create a file in this repository called `.env` (it will be ignored by git) with the following contents:
  ```
  USER_ID=[user id for user you just created]
  GROUP_ID=[group id for user you just created]
  DOMAIN_NAME=[local domain name]
  ```
- create a media folder in docker-compose's folder with $USER_ID:$GROUP_ID ownership
- get your Plex claim token at https://www.plex.tv/claim/
- run `PLEX_TOKEN="..." docker-compose up -d`

## Config


### Transmission

We use [Transmission](https://transmissionbt.com/) as the torrent downloader.

- stop transmission's container
- configure basic auth at `media/transmission/config/settings.json` (you will need to touch `rpc-authentication-required`, `rpc-username` and `rpc-password`)
- start transmission's container

### nzbGet

We use [nzbGet](https://nzbget.net/) as the Usenet downloader

- Add a usenet server, along with credentials from the Settings.
 

### Sickgear

We use [Sickgear](https://github.com/SickGear/SickGear.Docker) to track and manage TV shows.

- setup auto-update and authentication
- connect transmission as a downloader


### Radarr

We use [Radarr](https://radarr.video/) (a clone of Sonarr) to track and manage movies.

- setup auto-update and authentication
- connect transmission as a downloader


### Jackett

We use [Jackett](https://github.com/Jackett/Jackett) as a proxy between private trackers and our other components.

### Traefik

[Traefik](https://traefik.io/) is used as a reverse proxy
