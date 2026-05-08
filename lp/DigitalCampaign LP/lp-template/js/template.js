
 $("#btnSubmit").on('click',function(){
    $("#block0").hide();
    $("#block1_He").show();
  });

$("#popuphe").click(function () {
	$("#block1_He").hide();
	$("#block1").show();
});
$("#formSubscriptionSubmit").click(function () {
	$("#block1").hide();
	$("#block2").show();
    $("#footer_pin").show();
});
$("#formValidationSubmit").click(function () {
    $("#footer_pin").hide();
	$("#block2").hide();
	$("#block3").show();
});



function abrir1(){
  window.open ("#", "Janela", "status=no, width=950, height=600 , left=450 , top=250,  scrollbars=1")
};

$(document).ready(function() {

    window.onload = mainLng;
    function mainLng() {
        $(".lng_en").toggleClass("active");
        $("a.language span").toggleClass("active");
    }

});

function btnLng() {
    $(".lng_en").toggleClass("active");
    $(".lng_ar").toggleClass("active");
    if ($("a.language .lng_en").hasClass("active")) {
        $("option#carrier").addClass("ar").text("إختر المشغل لديك");
    } else {
        $("option#carrier").removeClass("ar").text("Choose your carrier");
    }
}