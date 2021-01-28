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
            this.slider.value = brightness;
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

    window.controlPopups = {
        BrightnessPopup: BrightnessPopup,
    }

})();
