import "./jquery-global";
import "bootstrap-cron-picker/dist/cron-picker";

let auth = "";
window.setauth = (a) => {
    auth = a;
};

$().ready(() => {
    $('#newSchedule').cronPicker();
    $('#newretafterstoreSchedule').cronPicker();

    $("#addDone").hide();
    if ($('#newSR').is(':checked')) {
        $("#newstorage").hide();
    } else {
        $("#newretrieval").hide();
    }
    $("#newSR").on('change', () => {
        if ($('#newSR').is(':checked')) {
            $("#newretrieval").show();
            $("#newstorage").hide();
        } else {
            $("#newretrieval").hide();
            $("#newstorage").show();
        } 
    })

    if (!$('#newRepeat').is(':checked')) {
        $("#setschedule").hide();
    }
    $("#newRepeat").on('change', () => {
        if ($('#newRepeat').is(':checked')) {
            $("#setschedule").show();
        } else {
            $("#setschedule").hide();
        } 
    })

    if (!$('#newRepeatRetAfterStore').is(':checked')) {
        $("#retafterstoreschedule").hide();
    }
    $("#newRepeatRetAfterStore").on('change', () => {
        if ($('#newRepeatRetAfterStore').is(':checked')) {
            $("#retafterstoreschedule").show();
        } else {
            $("#retafterstoreschedule").hide();
        } 
    })

    $("#addtask button").on('click', doSubmit);
    $("schedulesection form").on('submit', doSubmit);
})

function doSubmit(e) {
    if (e.preventDefault) {
        e.preventDefault()
    }

    // loop over miners
    let miners = $("#newMiner").val().trim().split('\n')

    let remaining = miners.length;
    let done = () => {
        remaining--;
        if (!remaining) {
            $("#addDone").show()
        }
    }

    username = undefined;
    password = undefined;
    if (auth != "") {
        let ap = auth.split(":");
        username = ap[0];
        password = ap[1];
    }

    for (let i = 0; i < miners.length; i++) {
        let miner = miners[i];
        let url = "/tasks/storage";
        let data = {};
        if ($('#newSR').is(':checked')) {
            url = "/tasks/retrieval";
            data = {
                "Miner": miner,
                "PayloadCID": $('#newCid').val(),
                "CARExport": false,
                "MaxPriceAttoFIL": 20000000000,
            }
            if ($('#newRepeatRetAfterStore').is(':checked')) {
                data.RetrievalSchedule = $('#newretafterstoreSchedule').val()
                data.RetrievalScheduleLimit = $('#newretafterstoreScheduleLimit').val()
            }
        } else {
            data = {
                "Miner": miner,
                "Size": $('#newSize').val(),
                "StartOffset": 6152, // 3 days?
                "FastRetrieval": $('#newFast').is(':checked'),
                "Verified": $('#newVerified').is(':checked'),
                "MaxPriceAttoFIL": 20000000000,
            }
        }

        if ($('#newRepeat').is(':checked')) {
            data.Schedule = $('#newSchedule').val()
            data.ScheduleLimit = $('#newScheduleLimit').val()
        }
        if ($('#newScheduleTag').val() !='') {
            data.Tag = $('#newScheduleTag').val()
        }

        $.ajax({
            type: "POST",
            url: url,
            data: data,
            username: username,
            password: password,
            success: done,
        });
    }

    return false
}
