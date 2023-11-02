document.addEventListener('DOMContentLoaded', function () {
  var checkIfStorybookRendered = setInterval(function () {
    var targetDiv = document.querySelectorAll("#panel-tab-content > div")[0];
    if (targetDiv) {
      clearInterval(checkIfStorybookRendered);
      var insertHTML = `
          <div 
            id="branding-customization-notification"
            style="
              padding: 30px 20px 20px;
              background: #ffff57;
              margin: 0;
              position:relative;
            "
          >
              <p style="
                  line-height: 1.45em;
                  font-size: 1.4em;
                ">An improved branding customization UI experience is available through the <b><code>auth0 universal-login customize</code></b> command.</p>
              <button 
                id="branding-close-button"
                style="
                  position: absolute;
                  top:0px;
                  right:0px;
                  background: transparent;
                  border: none;
                  padding: 10px 15px;
                  cursor:pointer;
                  text-decoration: underline;
                "
              >Close</button>
          </div>
          `;
      targetDiv.innerHTML = insertHTML + targetDiv.innerHTML;

      var brandingCloseButton = document.getElementById('branding-close-button');
      brandingCloseButton.addEventListener('click', function () {
        var brandingCustomizationNotification = document.getElementById('branding-customization-notification');
        if (brandingCustomizationNotification) {
          brandingCustomizationNotification.remove();
        }
      });
    }
  }, 100);
});
