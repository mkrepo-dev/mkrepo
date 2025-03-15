$(document).ready(updateUrl);
$("#name").on("input", updateUrl);
$("#owner").on("input", updateUrl);

function updateUrl() {
  var name = $("#name").val().trim().replace(/\s+/g, "-");
  var owner = $("#owner").val();
  $("#url-path").text(owner+"/"+name);
}

$(document).ready(updateLicenseVars);
$("#license").on("change", updateLicenseVars);

function updateLicenseVars() {
  var vars = $("#license option:selected").attr("vars").split(",");
  vars = vars.map(function(v) { return v.toLowerCase(); });
  var inputs = [$('#license-year'), $('#license-fullname'), $('#license-project')];
  inputs.forEach(function(input, i) {
    var name = input.attr('id').replace('license-', '');
    if (vars.includes(name)) {
      input.parent().show();
    } else {
      input.parent().hide();
    }
  });
}
