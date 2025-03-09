$(document).ready(updateUrl);
$("#name").on("input", updateUrl);
$("#owner").on("input", updateUrl);

function updateUrl() {
  var name = $("#name").val().replace(/\s+/g, "-");
  var owner = $("#owner").val();
  $("#url-path").text(owner+"/"+name);
}
