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

function rgbToHex(rgb) {
    let res = '#';
    rgb.forEach((x) => {
        if (x < 0x10) {
            res += '0';
        }
        res += x.toString(16);
    });
    return res;
}

function hexToRGB(hex) {
    const res = [0, 0, 0];
    for (let i = 0; i < 3; i++) {
        res[i] = parseInt(hex.substr(i * 2 + 1, 2), 16);
    }
    return res
}
