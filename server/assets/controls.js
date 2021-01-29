(function () {

    class ControlPopup {
        constructor() {
            this.element = makeElem('div', 'popup-container');
            this.dialog = makeElem('div', 'popup-window');
            this.dialog.className = 'popup-window'
            this.element.appendChild(this.dialog);

            this.buttons = makeElem('div', 'popup-buttons');
            this.cancelButton = makeElem('button', 'popup-button popup-button-cancel', {
                textContent: 'Cancel',
            });
            this.okButton = makeElem('button', 'popup-button popup-button-ok', {
                textContent: 'Save',
            });
            this.buttons.appendChild(this.cancelButton);
            this.buttons.appendChild(this.okButton);

            this.content = makeElem('div', 'popup-content');
            this.dialog.appendChild(this.content);
            this.dialog.appendChild(this.buttons);

            this.okButton.addEventListener('click', () => this.confirm());
            this.cancelButton.addEventListener('click', () => this.close());
            this.dialog.addEventListener('click', (e) => e.stopPropagation());
            this.element.addEventListener('click', () => this.close());
        }

        createContent() {
            // Override in a sub-class to append controls.
        }

        open() {
            document.body.appendChild(this.element);
        }

        confirm() {
            this.close();
        }

        close() {
            document.body.removeChild(this.element);
        }
    }

    class BrightnessPopup extends ControlPopup {
        constructor(brightness) {
            super();

            this.onBrightness = (_value) => null;

            this.slider = makeElem('input', 'popup-slider', {
                type: 'range',
                min: 1,
                max: 100,
                value: brightness,
            });
            this.content.appendChild(this.slider);

            this.label = makeElem('label', 'popup-slider-label', {
                textContent: brightness + "%",
            });
            this.content.appendChild(this.label);

            this.slider.addEventListener('input', () => {
                this.label.textContent = this.slider.value + '%';
            });
        }

        confirm() {
            super.confirm();
            this.onBrightness(parseInt(this.slider.value));
        }
    }

    class ColorPopup {
        constructor(status) {
            super();

            this.onRGB = (_value) => null;
            this.onTone = (_value) => null;

            this.tabs = makeElem('div', 'popup-tabs');
            this.toneTab = makeElem('button', 'popup-tabs-tab', { textContent: 'Tone' });
            this.rgbTab = makeElem('button', 'popup-tabs-tab', { textContent: 'RGB' });
            this.tabs.appendChild(this.toneTab);
            this.tabs.appendChild(this.rgbTab);
            this.content.appendChild(this.tabs);
            this.toneTab.addEventListener('click', () => this.showTab(false));
            this.rgbTab.addEventListener('click', () => this.showTab(true));

            this.tonePane = makeElem('div', 'popup-tab-pane');
            this.toneSlider = makeElem('input', 'popup-slider', {
                type: 'range',
                min: 0,
                max: 100,
                value: 50,
            });
            this.toneLabel = makeElem('input', 'popup-slider-label', { textContent: '50%' });
            this.toneSlider.addEventListener('input', () => {
                this.toneLabel.textContent = this.toneSlider.value + '%';
            });
            this.tonePane.appendChild(this.toneSlider);
            this.tonePane.appendChild(this.toneLabel);
            this.content.appendChild(this.tonePane);

            this.rgbPane = makeElem('div', 'popup-tab-pane');
            this.rgbInput = makeElem('input', 'popup-color-picker', {
                type: 'color',
                value: '#00ff00',
            });
            this.rgbPane.appendChild(this.rgbInput);
            this.content.appendChild(this.rgbPane);

            this.useRGB = status['use_rgb'];
            if (this.useRGB) {
                this.rgbInput.value = rgbToHex(status['rgb']);
            } else {
                this.toneSlider.value = status['color_tone'];
                this.toneLabel.value = status['color_tone'] + '%';
            }
            this.showTab(status['use_rgb']);
        }

        showTab(useRGB) {
            this.useRGB = useRGB;
            this.toneTab.classList.remove('popup-tabs-selected');
            this.tonePane.classList.remove('popup-tab-pane-current');
            this.rgbTab.classList.remove('popup-tabs-selected');
            this.rgbPane.classList.remove('popup-tab-pane-current');
            if (this.useRGB) {
                this.rgbTab.classList.add('popup-tabs-selected');
                this.rgbPane.classList.add('popup-tab-pane-current');
            } else {
                this.toneTab.classList.add('popup-tabs-selected');
                this.tonePane.classList.add('popup-tab-pane-current');
            }
        }

        confirm() {
            super.confirm();
            if (this.useRGB) {
                this.onRGB(hexToRGB(this.rgbInput.value));
            } else {
                this.onTone(parseInt(this.toneSlider.value));
            }
        }
    }

    window.controlPopups = {
        BrightnessPopup: BrightnessPopup,
        ColorPopup: ColorPopup,
    }

})();
