$(document).ready(updateUrl);
$("#name").on("input", updateUrl);
$("#owner").on("input", updateUrl);

function updateUrl() {
  var name = $("#name").val().trim().replace(/\s+/g, "-");
  var path = $("#owner option:selected").attr("path");
  $("#url-path").text(path+"/"+name);
}

var schemas = null;

$.ajax({
  url: "/schemas",
  type: "GET",
  success: function(data) {
    schemas = data;
  }
});

$(document).ready(updateLicenseFields);
$("#license").on("change", updateLicenseFields);

function getType(typeValue) {
  if (Array.isArray(typeValue)) return typeValue[0];
  return typeValue || "string";
}

function renderFieldsFromSchema(schema) {
  if (!schema || !schema.properties) return "";
  var html = "";
  for (var groupName in schema.properties) {
    var groupSchema = schema.properties[groupName];
    if (!groupSchema || !groupSchema.properties) continue;
    var required = groupSchema.required || [];
    for (var fieldName in groupSchema.properties) {
      var fieldSchema = groupSchema.properties[fieldName];
      if (!fieldSchema) continue;
      var isRequired = required.indexOf(fieldName) !== -1;
      var type = getType(fieldSchema.type);
      var title = fieldSchema.title || fieldName;
      var inputName = "license-" + fieldName;
      var defaultVal = fieldSchema.default !== undefined ? fieldSchema.default : "";
      var reqAttr = isRequired ? " required" : "";
      var defaultAttr = defaultVal !== "" ? ' value="' + defaultVal + '"' : "";
      var placeholderAttr = fieldSchema.description ? ' placeholder="' + fieldSchema.description + '"' : "";

      html += '<div class="mb-3">';
      html += '<label for="' + inputName + '" class="form-label mb-0"><small><b>' + title;
      if (isRequired) html += ' <span class="text-danger">*</span>';
      html += "</b></small></label>";

      if (fieldSchema.enum) {
        html += '<select class="form-select" id="' + inputName + '" name="' + inputName + '"' + reqAttr + ">";
        html += '<option value="">-- select --</option>';
        for (var i = 0; i < fieldSchema.enum.length; i++) {
          html += '<option value="' + fieldSchema.enum[i] + '">' + fieldSchema.enum[i] + "</option>";
        }
        html += "</select>";
      } else if (type === "boolean") {
        html += '<input type="checkbox" class="form-check-input" id="' + inputName + '" name="' + inputName + '" value="true"' + reqAttr + ">";
      } else if (type === "integer") {
        html += '<input type="number" step="1" class="form-control" id="' + inputName + '" name="' + inputName + '"' + defaultAttr + placeholderAttr + reqAttr + ">";
      } else if (type === "number") {
        html += '<input type="number" class="form-control" id="' + inputName + '" name="' + inputName + '"' + defaultAttr + placeholderAttr + reqAttr + ">";
      } else {
        html += '<input type="text" class="form-control" id="' + inputName + '" name="' + inputName + '"' + defaultAttr + placeholderAttr + reqAttr + ">";
      }

      if (fieldSchema.description) {
        html += '<div class="form-text">' + fieldSchema.description + "</div>";
      }
      html += "</div>";
    }
  }
  return html;
}

function updateLicenseFields() {
  var selected = $("#license").val();
  $("#license-fields-container").empty();
  if (selected && schemas && schemas.licenses && schemas.licenses[selected] && schemas.licenses[selected].schema) {
    var html = renderFieldsFromSchema(schemas.licenses[selected].schema);
    if (html) {
      $("#license-fields-container").html(html);
      $("#license-schema-fields").show();
      return;
    }
  }
  $("#license-schema-fields").hide();
}

$(document).ready(updateInitTag);
$("#readme").on("change", updateInitTag);
$("#gitignore").on("change", updateInitTag);
$("#license").on("change", updateInitTag);

function updateInitTag() {
  if (!$("#readme").prop("checked")
    && ($("#gitignore").val() == "")
    && ($("#license").val() == "")
  ) {
    $("#tag").prop("disabled", true);
    $("#tag").prop("checked", false);
  } else {
    $("#tag").prop("disabled", false);
  }
}

// Reset modal input when modal is opened
$("#choose-template-btn").on("click", function() {
  $("#template-search").val("");
});

$("#template-search").on("input", debounce(handleTemplateSearch, 300));
$("#choose-template-btn").on("click", function() {
  templateSearch("");
})

function debounce(func, delay) {
  let timer;
  return function(...args) {
    clearTimeout(timer);
    timer = setTimeout(function() {func.apply(this, args)}, delay);
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
                ${item.buildIn ? `<span class="badge rounded-pill text-bg-success">official</span>` : ""}
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
        var idx = parseInt($(this).attr("id").replace("template-", ""), 10);
        applyTemplate(data[idx]);
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
