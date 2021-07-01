import "./jquery-global";
import "bootstrap-table";
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

    syncTable();
})

function syncTable() {
    let username = undefined;
    let password = undefined;
    if (auth != "") {
        let ap = auth.split(":");
        username = ap[0];
        password = ap[1];
    }
    $.ajax({
        type: "GET",
        url: "./tasks",
        username: username,
        password: password,
        success: gotTable,
    });
}

function operate(val, row) {
    return '<a title="remove">Cancel</a>';
}

function gotTable(data) {
    for (let i = 0; i < data.length; i++) {
        data[i].task = data[i].StorageTask || data[i].RetrievalTask;
        data[i].sched = {
            Schedule: data[i].task.Schedule,
            Limit: data[i].task.ScheduleLimit,
        }
        delete data[i].task.Schedule;
        delete data[i].task.ScheduleLimit;
    }
    let stringify = (d) => JSON.stringify(d, null, 2);
    console.log(data);
    $("#taskTable").bootstrapTable({
        idField: 'UUID',
        columns: [
            {title:'ID', field:'UUID'},
            {title:'Status', field:'Status'},
            {title:'Task', field:'task', formatter: stringify},
            {title:'Schedule', field:'sched', formatter: stringify},
            {title:'Delete', field: 'operate', align: 'center', formatter: operate}
        ],
        data: data,
    });
}

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

    let username = undefined;
    let password = undefined;
    if (auth != "") {
        let ap = auth.split(":");
        username = ap[0];
        password = ap[1];
    }

    for (let i = 0; i < miners.length; i++) {
        let miner = miners[i];
        let url = "./tasks/storage";
        let data = {};
        if ($('#newSR').is(':checked')) {
            url = "./tasks/retrieval";
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
                "Size": Number($('#newSize').val()),
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
            contentType: "application/json",
            url: url,
            data: JSON.stringify(data),
            username: username,
            password: password,
            success: done,
        });
    }

    return false
}
