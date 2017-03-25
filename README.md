# EventDB

Simple database for keeping some events from various sources.
Dedicated to use as source for Grafana (json) and webhook for
Prometheus Alert Manager.


## Building and running

### Dependency

* github.com/prometheus/client_golang/
* github.com/prometheus/common
* github.com/boltdb/bolt
* gopkg.in/yaml.v2


### Local Build & Run

    go build
    ./eventdb

Configuration file: `eventdb.yml`


# License
Copyright (c) 2017, Karol Będkowski.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
