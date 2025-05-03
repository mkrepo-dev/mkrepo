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
  if ($("#dockerfile").val() == "") {
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
    && ($("#gitignore").val() == "")
    && ($("#dockerfile").val() == "")
    && ($("#license").val() == "")
  ) {
    $("#tag").prop("disabled", true);
    $("#tag").prop("checked", false);
  } else {
    $("#tag").prop("disabled", false);
  }
}

$("#template-search").on("input", debounce(handleTemplateSearch, 300));
$("#choose-template-btn").on("click", () => {
  templateSearch("");
})

function debounce(func, delay) {
  let timer;
  return function (...args) {
    clearTimeout(timer);
    timer = setTimeout(() => func.apply(this, args), delay);
  };
}

function handleTemplateSearch(event) {
  const query = event.target.value;
  if (query.trim() === "" || query.length == 1) {
    return;
  }
  templateSearch(query);
}

function templateSearch(query) {
  $.ajax({
    url: "/templates",
    type: "GET",
    data: { q: query },
    success: function(data) {
      $('#template-results').empty();
      data.forEach(function(item, index) {
        $('#template-results').append(`
          <div class="list-group-item list-group-item-action template" style="cursor: pointer" data-bs-dismiss="modal" id="template-${index}">
            <div class="d-flex w-100 justify-content-between">
              <h5 class="mb-1">${item.name}</h5>
              <div>
                ${item.buildIn ? `<span class="badge rounded-pill text-bg-success">build in</span>` : ""}
                ${item.language ? `<span class="badge rounded-pill text-bg-info">${item.language}</span>` : ""}
                <span class="badge rounded-pill text-bg-light">${item.version}</span>
              </div>
            </div>
            ${!item.buildIn ? `<small><a href="${item.url}">${item.fullName}</a></small>` : ""}
            ${item.description ? `<p class="card-text">${item.description}</p>` : ""}
          </div>
        `);
      });
      $(".template").on("click", function() {
        applyTemplate(data[$(this).attr("id").split("-")[1]]);
      });
    },
    error: function(xhr, status, error) {
      $('#template-results').html('<p>Error loading data</p>');
    }
  })
}

function applyTemplate(template) {
  $("#template").val(`${template.fullName}@${template.version}`);
  $("#choose-template-btn").hide();
  const templateBtnWrapper = $("#selected-template-btn");
  templateBtnWrapper.show();
  const templateBtn = templateBtnWrapper.find("button");
  templateBtn.html(`<b>Template:</b> ${template.name} <i class="bi bi-x-lg ms-2" style="font-size: 1rem; color: red;"></i>`);
  templateBtn.on("click", function() {
    clearTemplate();
  });
}

function clearTemplate() {
  $("#template").val("");
  $("#choose-template-btn").show();
  const templateBtnWrapper = $("#selected-template-btn");
  templateBtnWrapper.hide();
  const templateBtn = templateBtnWrapper.find("button");
  templateBtn.text("");
}
