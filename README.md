# Quick Start

``` shell
go install github.com/ProgKea/lose@latest
lose -index <path> -serve
```

After that lose will create a `lose.index` file and start a webserver on port 8080.

![lose demo](lose_demo.gif "lose demo")

There is also an experimental fuzzy searching. It doesn't look for a exact term match but a fuzzy match:

![fuzzy lose demo](fzy_lose_demo.gif "fuzzy lose demo")

# Useful Resources

- blog post explaining tfidf and cosine similarity: https://janav.wordpress.com/2013/10/27/tf-idf-and-cosine-similarity/
- snowball: https://github.com/snowballstem/snowball
