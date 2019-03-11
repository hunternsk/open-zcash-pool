# Open Zcash Pool

### Features
✓ Easy to install and setup\
✓ Solo mining (with many rigs)\
✓ Frontend with:\
&emsp;✓ Network stats\
&emsp;✓ Miner stats\
&emsp;✓ Mined blocks\
✗ Multi-miner mining (with many miners)\
✗ Unlocker\
✗ Payouts

### Building on Linux

#### Dependencies:

- go >= 1.9
- zcashd = 2.0.2
- 2.8.0 <= redis-server <= 4.0.12
- 4 LTS <= nodejs <= 10 LTS
- nginx

**Highly recomended Ubuntu 16.04 LTS or 18.04 LTS.** You can try other versions, but we don't take responsibility.

#### Clone OZP repository, compile equihash lib & pool

```sh
$ cd ~
$ git clone https://github.com/JKKGBE/open-zcash-pool.git
$ cd open-zcash-pool/equihash/libs
$ make
$ cd ../..
$ make
```

#### Install redis-server

```sh
$ cd ~
$ wget http://download.redis.io/releases/redis-4.0.12.tar.gz
$ tar xzf redis-4.0.12.tar.gz
$ cd redis-4.0.12
$ make
```

#### Install zcashd

First install the following dependency so you can talk to zcash repository using HTTPS:

```sh
$ sudo apt install -y apt-transport-https wget gnupg2
```

Next add the Zcash master signing key to apt's trusted keyring:

```sh
$ wget -qO - https://apt.z.cash/zcash.asc | sudo apt-key add -
```

`Key fingerprint = 3FE6 3B67 F85E A808 DE9B  880E 6DEF 3BAF 2727 66C0`

Add the repository to your sources:

```sh
$ echo "deb [arch=amd64] https://apt.z.cash/ jessie main" | sudo tee /etc/apt/sources.list.d/zcash.list
```

Finally, update the cache of sources and install Zcash:

```sh
$ sudo apt update && sudo apt install -y zcash
```

In order to connect to the test network, you can use this command to create `zcash.conf` in ~/.zcash directory:

```sh
$ echo -e "testnet=1
addnode=testnet.z.cash
rpcuser=yourZcashNodeUsername
rpcpassword=yourZcashNodePassword" > ~/.zcash/zcash.conf
```

In order to connect to the main network, you can use this command to create `zcash.conf` in ~/.zcash directory:

```sh
$ echo -e "addnode=mainnet.z.cash
rpcuser=yourZcashNodeUsername
rpcpassword=yourZcashNodePassword" > ~/.zcash/zcash.conf
```

Make sure to change "yourZcashNodeUsername" and "yourZcashNodePassword" in the aforementioned file after creating it. You can use:

```sh
$ nano ~/.zcash/zcash.conf
```

When you're done press CTRL + X, type "y" and press Enter.

You can now run Zcash Daemon (Node) by typing:
```sh
$ zcashd
```

Wait until the Zcash Daemon has downloaded all the blocks and proceed to the next step.

### Running Pool

Our example config has default ports for redis-server and zcashd.

Create config.json by copying config.example.json:

```sh
$ cd ~/open-zcash-pool
$ cp config.example.json config.json
```

After editing config.json, run the pool using:

```sh
$ ./build/bin/open-zcash-pool config.json
```

Fields explanation:

```javascript
{
    // How many CPU threads should your pool use
    "threads": 2,
    // Prefix for keys in redis store
    "coin": "zec",
    // Give unique name to each instance
    "name": "main",
    // Unique id for each instance
    "instanceId": 1,
    // Change to your Zcash t-address
    "poolAddress": "tmGoHHqgsCRuEna9YQX9zKp9ujeqGLMLEYi",

    "proxy": {
        "enabled": true,
        // Will be removed later, doesn't matter
        "listen": "0.0.0.0:8888",
        "limitHeadersSize": 1024,
        "limitBodySize": 256,
        /*
            Set to true if you are behind CloudFlare (not recommended) or behind http-reverse
            proxy to enable IP detection from X-Forwarded-For header.
            Advanced users only. It's tricky to make it right and secure.
        */
        "behindReverseProxy": false,
        // How often should pool ask Zcash Daemon for new work
        "blockRefreshInterval": "120ms",
        "stateUpdateInterval": "3s",
        // Difficulty for shares - 256 for CPU or testing, 4096 for 1 GPU, 32768 for 6 GPU and more
        "difficulty": 256,
        // TTL for workers stats, usually should be equal to large hashrate window from API section
        "hashrateExpiration": "3h",

        /*
            Reply error to miner instead of job if redis is unavailable.
            Should save electricity to miners if pool is sick and they didn't set up failovers.
        */
        "healthCheck": true,
        // Mark pool sick after this number of redis failures.
        "maxFails": 100,

        // Stratum mining endpoint
        "stratum": {
            "enabled": true,
            // Bind stratum mining socket to this IP:PORT
            "listen": "0.0.0.0:8008",
            "timeout": "120s",
            "maxConn": 8192
        },

        "policy": {
            "workers": 8,
            "resetInterval": "60m",
            "refreshInterval": "1m",

            "banning": {
                "enabled": false,
                /*
                    Name of ipset for banning.
                    Check http://ipset.netfilter.org/ documentation.
                */
                "ipset": "blacklist",
                // Remove ban after this amount of time
                "timeout": 1800,
                // Percent of invalid shares from all shares to ban miner
                "invalidPercent": 30,
                // Check after after miner submitted this number of shares
                "checkThreshold": 30,
                // Bad miner after this number of malformed requests
                "malformedLimit": 5
            },
            // Connection rate limit
            "limits": {
                "enabled": false,
                // Number of initial connections
                "limit": 30,
                "grace": "5m",
                // Increase allowed number of connections on each valid share
                "limitJump": 10
            }
        }
    },

    // Provides JSON data for frontend which is static website
    "api": {
        "enabled": true,
        /*
            If you are running API node on a different server where this module
            is reading data from redis writeable slave, you must run an api instance with this option enabled in order to purge hashrate stats from main redis node.
            Only redis writeable slave will work properly if you are distributing using redis slaves.
            Very advanced. Usually all modules should share same redis instance.
        */
        "purgeOnly": false,
        // Purge stale stats interval
        "purgeInterval": "10m",
        "listen": "0.0.0.0:8080",
        // Collect miners stats (hashrate, ...) in this interval
        "statsCollectInterval": "5s",
        // Fast hashrate estimation window for each miner from it's shares
        "hashrateWindow": "30m",
        // Long and precise hashrate from shares, 3h is cool, keep it
        "hashrateLargeWindow": "3h",
        // Collect stats for shares/diff ratio for this number of blocks
        "luckWindow": [64, 128, 256],
        // Max number of payments to display in frontend
        "payments": 30,
        // Max numbers of blocks to display in frontend
        "blocks": 50
    },

    "upstreamCheckInterval": "5s",

    /*
        List of zcashd nodes to poll for new jobs. Pool will try to get work from
        first alive one and check in background for failed to back up.
        Current block template of the pool is always cached in RAM indeed.
    */
    "upstream": [
        {
            "name": "main",
            // Change this to values from ~/.zcash/zcash.conf
            "url": "http://yourZcashNodeUsername:yourZcashNodePassword@127.0.0.1:18232",
            "timeout": "10s"
        },
        {
            "name": "backup",
            // Change this to values from ~/.zcash/zcash.conf
            "url": "http://yourZcashNodeUsername:yourZcashNodePassword@127.0.0.2:18232",
            "timeout": "10s"
        }
    ],

    // This is standard redis connection options
    "redis": {
        // Where your redis instance is listening for commands
        "endpoint": "127.0.0.1:6379",
        "poolSize": 10,
        "database": 0,
        "password": ""
    }
}
```

### Building Frontend

Install nodejs. I suggest using LTS version >= 4.x from https://github.com/nodesource/distributions or from your Linux distribution or simply install nodejs on Ubuntu 16.04.

The frontend is a single-page Ember.js application that polls the pool API to render miner stats.

```sh
$ cd www
```

Change <code>ApiUrl: '//example.net/'</code> in <code>www/config/environment.js</code> to match your domain name. Also don't forget to adjust other options.

You might have to run commands with "-g" with sudo.

```sh
$ npm install -g ember-cli@2.9.1
$ npm install -g bower
$ npm install
$ bower install
$ ./build.sh
```

Configure nginx to serve API on <code>/api</code> subdirectory.
Configure nginx to serve <code>www/dist</code> as static website.

#### Serving API using nginx

Create an upstream for API:

```
upstream api {
    server 127.0.0.1:8080;
}
```

and add this setting after <code>location /</code>:

```
location /api {
    proxy_pass http://api;
}
```

#### Customization

You can customize the layout using built-in web server with live reload:

```sh
$ ember server --port 8082 --environment development
```

**Don't use built-in web server in production**.

Check out <code>www/app/templates</code> directory and edit these templates
in order to customise the frontend.

### Credits

Made by:\
[Jakub Kowalski](https://github.com/JKKGBE)\
[Krzysztof Michalak](https://github.com/kmich1)\
[Jakub Kowalewski](https://github.com/chinesespall)

Thanks to:\
[sammy007](https://github.com/sammy007)

Licensed under GPLv3.

### Donations

ETH/ETC: 0x58910B01fe047A3064604C4D950EAbf97E6a5c70
