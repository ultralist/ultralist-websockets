Ultralist webhooks coordinator.  Uses redis and coordinates webhook channels.

Env vars:
* `DEBUG`: if set, will output more verbose logging statements
* `PORT`: the port to run on.  defaults to `8080`
* `REDIS_URL`: the url that redis is on
