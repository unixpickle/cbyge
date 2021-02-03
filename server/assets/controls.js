(function () {

    class ControlPopup {
        constructor() {
            const content = this.createContent();
            this.cancelButton = makeElem('button', 'popup-button popup-button-cancel', {
                textContent: 'Cancel',
            });
            this.okButton = makeElem('button', 'popup-button popup-button-ok', {
                textContent: 'Save',
            });
            this.buttons = makeElem('div', 'popup-buttons', {}, [this.cancelButton, this.okButton]);
            this.content = makeElem('div', 'popup-content', {}, content);
            this.dialog = makeElem('div', 'popup-window', {}, [this.content, this.buttons]);
            this.element = makeElem('div', 'popup-container', {}, [this.dialog]);

            this.dialog.addEventListener('click', (e) => e.stopPropagation());
            this.element.addEventListener('click', () => this.close());
            this.okButton.addEventListener('click', () => this.confirm());
            this.cancelButton.addEventListener('click', () => this.close());
        }

        createContent() {
            // Override in a sub-class.
            return [];
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

            this.dialog.classList.add('popup-window-small');
            this.onBrightness = (_value) => null;
        }

        createContent() {
            this.slider = makeElem('input', 'popup-slider', {
                type: 'range',
                min: 1,
                max: 100,
                value: brightness,
            });
            this.label = makeElem('label', 'popup-slider-label', {
                textContent: brightness + "%",
            });
            this.slider.addEventListener('input', () => {
                this.label.textContent = this.slider.value + '%';
            });
            return [this.slider, this.label];
        }

        confirm() {
            super.confirm();
            this.onBrightness(parseInt(this.slider.value));
        }
    }

    class ColorPopup extends ControlPopup {
        constructor(status) {
            super();

            this.onRGB = (_value) => null;
            this.onTone = (_value) => null;

            this.useRGB = status['use_rgb'];
            if (this.useRGB) {
                this.rgbInput.value = rgbToHex(status['rgb']);
            } else {
                this.toneSlider.value = status['color_tone'];
                this.toneLabel.textContent = status['color_tone'] + '%';
            }
            this.showTab(status['use_rgb']);
        }

        createContent() {
            this.toneTab = makeElem('button', 'popup-tabs-tab', { textContent: 'Tone' });
            this.rgbTab = makeElem('button', 'popup-tabs-tab', { textContent: 'RGB' });
            this.tabs = makeElem('div', 'popup-tabs', {}, [this.toneTab, this.rgbTab]);
            this.toneTab.addEventListener('click', () => this.showTab(false));
            this.rgbTab.addEventListener('click', () => this.showTab(true));

            this.toneSlider = makeElem('input', 'popup-slider', {
                type: 'range',
                min: 0,
                max: 100,
                value: 50,
            });
            this.toneLabel = makeElem('label', 'popup-slider-label', { textContent: '50%' });
            this.toneSlider.addEventListener('input', () => {
                this.toneLabel.textContent = this.toneSlider.value + '%';
            });
            this.tonePane = makeElem('div', 'popup-tab-pane', {}, [
                this.toneSlider, this.toneLabel,
            ]);

            this.rgbInput = makeElem('input', 'popup-color-picker', {
                type: 'color',
                value: '#00ff00',
            });
            this.rgbPane = makeElem('div', 'popup-tab-pane', {}, [this.rgbInput]);

            return [this.tabs, this.tonePane, this.rgbPane];
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
