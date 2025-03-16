$(document).ready(updateUrl);
$("#name").on("input", updateUrl);
$("#owner").on("input", updateUrl);

function updateUrl() {
  var name = $("#name").val().trim().replace(/\s+/g, "-");
  var path = $("#owner option:selected").attr("path");
  $("#url-path").text(path+"/"+name);
}

$(document).ready(updateDockerIgnore);
$("#dockerfile").on("change", updateDockerIgnore);

function updateDockerIgnore() {
  if ($("#dockerfile").val() == "none") {
    $("#dockerignore").prop("disabled", true);
    $("#dockerignore").prop("checked", false);
  } else {
    $("#dockerignore").prop("disabled", false);
  }
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

$(document).ready(updateInitTag);
$("#readme").on("change", updateInitTag);
$("#gitignore").on("change", updateInitTag);
$("#dockerfile").on("change", updateInitTag);
$("#license").on("change", updateInitTag);

function updateInitTag() {
  if (!$("#readme").prop("checked")
    && ($("#gitignore").val() == "none")
    && ($("#dockerfile").val() == "none")
    && ($("#license").val() == "none")
  ) {
    $("#tag").prop("disabled", true);
    $("#tag").prop("checked", false);
  } else {
    $("#tag").prop("disabled", false);
  }
}
