function makeElem(elem, className, attrs) {
    const res = document.createElement(elem);
    res.className = className;
    if (attrs) {
        Object.keys(attrs).forEach((key) => {
            res[key] = attrs[key];
        });
    }
    return res;
}
