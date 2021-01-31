const TONE_0 = [255, 196, 0];
const TONE_50 = [255, 255, 128];
const TONE_100 = [166, 234, 245];

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

function toneColor(toneValue) {
    let color1 = TONE_0;
    let color2 = TONE_50;
    let frac = toneValue / 50
    if (toneValue > 50) {
        frac = (toneValue - 50) / 50;
        color1 = TONE_50;
        color2 = TONE_100;
    }
    const rgb = color1.map((x0, i) => {
        const x1 = color2[i];
        return Math.round(frac * x1 + (1 - frac) * x0);
    });
    return rgbToHex(rgb);
}
