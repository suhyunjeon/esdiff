# Diff for Elasticsearch

by https://github.com/olivere/esdiff


The `esdiff` tool iterates over two indices in Elasticsearch 5.x or 6.x or 7.x

You need Go 1.11 or later to compile. Install with:

```
$ go install github.com/olivere/esdiff
```

## Example usage

First, we need to setup two Elasticsearch clusters for testing,
then seed a few documents.

```
# Create an Elasticsearch 5.x
# http://localhost:19200 and http://localhost:19201
# Create an Elasticsearch 6.x
# http://localhost:29200 and http://localhost:29201
# Create an Elasticsearch 7.x 
# http://localhost:39200 and http://localhost:39201

$ mkdir -p data
$ docker-compose -f docker-compose.yml up -d

Creating esdiff_elasticsearch5_1 ... done
Creating esdiff_elasticsearch7_1 ... done
Creating esdiff_elasticsearch6_1 ... done

# Add some documents
# es 5
$ ./seed/es5.sh
# es 6
$ ./seed/es6.sh
# es 7
$ ./seed/es7.sh

# Compile
$ go build
```

Let's make a simple diff:

```
$ ./esdiff -u=true 'http://localhost:39200/newindex/_doc' 'http://localhost:39200/oldindex/_doc'
Updated	1	{*diff.Document}.Source["id"]:
	-: "1"
	+: "2"
{*diff.Document}.Source["name"]:
	-: "Same Document"
	+: "New Document 2"

Deleted	2
```

