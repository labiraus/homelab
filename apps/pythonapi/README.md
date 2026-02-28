# pythonapi

## Building and running your application

First create the image:

`docker build --no-cache -f ./apps/pythonapi/dockerfile -t pythonapi:latest ./apps`

Check it's present:

`docker images`

Then deploy:

`docker run -d -p 8080:80 pythonapi:latest`

Then it can be tagged:

`docker tag pythonapi ghcr.io/labiraus/homelab/pythonapi:latest`

Then it can be pushed to registery:

`docker push ghcr.io/labiraus/homelab/pythonapi:latest`
