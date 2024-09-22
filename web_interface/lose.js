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

        const TAGS = ["canvas", "h1", "h2", "h3", "p", "a", "div", "span", "select", "td", "tr", "b"];
        for (let tagName of TAGS) {
            window[tagName] = (...children) => tag(tagName, ...children);
        }
    }

    // Util
    {
        ////////////////////////////////
        // Element
        function createRange(node, targetPos) {
            let range = document.createRange();
            range.selectNode(node);
            range.setStart(node, Math.max(0, targetPos-1));

            let pos = 0;
            const stack = [node];
            while (stack.length > 0) {
                const current = stack.pop();

                if (current.nodeType === Node.TEXT_NODE) {
                    const len = current.textContent.length;
                    if (pos + len >= targetPos) {
                        range.setEnd(current, targetPos - pos);
                        return range;
                    }
                    pos += len;
                } else if (current.childNodes && current.childNodes.length > 0) {
                    for (let i = current.childNodes.length - 1; i >= 0; i--) {
                        stack.push(current.childNodes[i]);
                    }
                }
            }

            // The target position is greater than the
            // length of the contenteditable element.
            range.setEnd(node, node.childNodes.length);
            return range;
        };

        Element.prototype.getCursor = function() {
            const selection = window.getSelection();
            let position = 0;

            if (selection.rangeCount > 0) {
                const range = selection.getRangeAt(0);
                const preCaretRange = range.cloneRange();
                preCaretRange.selectNodeContents(this);
                preCaretRange.setEnd(range.endContainer, range.endOffset);
                position = preCaretRange.toString().length;
            }

            return position;
        }

        Element.prototype.setCursor = function(pos) {
            const range = document.createRange();
            const selection = window.getSelection();
            let charIndex = 0;
            let foundStart = false;

            function traverseNodes(node) {
                if (foundStart) return;

                if (node.nodeType === Node.TEXT_NODE) {
                    const nextCharIndex = charIndex + node.length;
                    if (pos >= charIndex && pos <= nextCharIndex) {
                        range.setStart(node, pos - charIndex);
                        range.collapse(true);
                        foundStart = true;
                    }
                    charIndex = nextCharIndex;
                } else if (node.nodeType === Node.ELEMENT_NODE) {
                    for (let i = 0; i < node.childNodes.length; i++) {
                        traverseNodes(node.childNodes[i]);
                        if (foundStart) return;
                    }
                }
            }

            traverseNodes(this);
            selection.removeAllRanges();
            selection.addRange(range);
        }

        ////////////////////////////////
        // Range

        class Rng {
            constructor(begin, end) {
                this.begin = begin;
                this.end = end;
            }
        }

        ////////////////////////////////
        // String

        String.prototype.insert = function(index, string) {
            return this.substring(0, index) + string + this.substring(index, this.length);
        }

        function stringRangesFromString(string) {
            if (typeof string !== "string") {
                return;
            }

            const result = [];

            const stringLength = string.length;
            for (let i = 0; i < stringLength; ++i) {
                if (string[i] == '"' || string[i] == '\'') {
                    const begin = i;

                    i += 1;
                    let found = false;
                    for (; i < stringLength; ++i) {
                        if (string[i] == '"' || string[i] == '\'') {
                            i += 1;
                            const end = i;
                            result.push(new Rng(begin, end));
                            found = true;
                            break;
                        }
                    }

                    if (!found) {
                        result.push(new Rng(begin, stringLength));
                        break;
                    }
                }
            }

            console.log(result);

            return result;
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

    searchInput.oninput = function (e) {
        const ranges = stringRangesFromString(this.innerText);
        let coloredHtml = this.innerText;

        const searchInputClone = searchInput.cloneNode();
        {
            let idx = 0;
            for (const range of ranges) {
                searchInputClone.push(this.innerText.substring(idx, range.begin))
                searchInputClone.push(span(this.innerText.substring(range.begin, range.end)).pushClass("string-literal"));
                idx = range.end;
            }

            if (idx != this.innerText.length) {
                searchInputClone.push(this.innerText.substring(idx, this.innerText.length));
            }
        }

        const cursorBefore = this.getCursor();
        searchInput.innerHTML = searchInputClone.innerHTML;
        this.setCursor(cursorBefore);
    }

    searchInput.onkeypress = async function (e) {
        if (e.key === "Enter") {
            e.preventDefault();
            const response = await fetch(`/search?query=${encodeURI(this.innerText)}`, {
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
