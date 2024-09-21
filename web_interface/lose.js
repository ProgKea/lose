(function () {
    // Builder Code
    {
        Element.prototype.push = function(...children) {
            children.forEach(child => {
                if (typeof child !== "object") {
                    this.appendChild(document.createTextNode(child.toString()))
                } else {
                    this.appendChild(child)
                }
            });
        }

        function tag(name, ...children) {
            const element = document.createElement(name)

            element.push(...children)

            element.setAttr = function(attributes) {
                for (const key in attributes) {
                    this.setAttribute(key, attributes[key]);
                }
                return this;
            }

            element.setClickFunc = function(func) {
                this.onclick = func;
                return this;
            }

            element.pushClass = function(...classNames) {
                classNames.forEach(className => {
                    this.classList.add(className);
                });
                return this;
            }

            element.setId = function(id) {
                this.id = id;
                return this;
            }

            return element
        }

        const TAGS = ["canvas", "h1", "h2", "h3", "p", "a", "div", "span", "select", "td", "tr"];
        for (let tagName of TAGS) {
            window[tagName] = (...children) => tag(tagName, ...children);
        }
    }

    function addSearchResult(entry) {
        searchResults.push(
            tr(
                td(entry.filepath)
                    .pushClass("copyable")
                    .setClickFunc(function (e) {
                        navigator.clipboard.writeText(this.innerText);
                    }),
                td(entry.score),
            ),
        )
    }

    function notFound() {
        return p("No results :(");
    }

    searchInput.onkeypress = async function (e) {
        if (e.key === "Enter") {
            e.preventDefault();
            const response = await fetch(`/search?needle=${encodeURI(searchInput.value)}`, {
                method: "GET",
            });
            const results = await response.json();
            searchResults.innerHTML = "";
            if (results[0].score == 0) {
                searchResults.push(notFound());
            } else {
                for (const result of results) {
                    if (result.score === 0) {
                        break;
                    }
                    addSearchResult(result);
                }
            }
        }
    };
})();
