# Quick Start

``` shell
go install github.com/ProgKea/lose
lose -index <path> -serve
```

After that lose will create a `lose.index` file and start a webserver on port 8080.

![lose demo](lose_demo.gif "lose demo")

There is also an experimental fuzzy searching. It doesn't look for a exact term match but a fuzzy match:

![fuzzy lose demo](fzy_lose_demo.gif "fuzzy lose demo")
