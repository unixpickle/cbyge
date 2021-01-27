(function () {

    class ControlPopup {
        constructor() {
            this.onConfirm = () => null;

            this.element = document.createElement('div');
            this.element.className = 'popup-container';
            this.dialog = document.createElement('div');
            this.dialog.className = 'popup-window'
            this.element.appendChild(this.dialog);

            this.element.addEventListener('click', () => this.close());

            this.createContent();

            // TODO: create cancel/ok button here.
        }

        createContent() {
            // Override in a sub-class to append controls.
        }

        open() {
            document.body.appendChild(this.element);
        }

        close() {
            document.body.removeChild(this.element);
        }
    }

})();
