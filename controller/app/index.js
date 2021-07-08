import "./jquery-global";
import "bootstrap-table";
import "bootstrap-cron-picker/dist/cron-picker";

let auth = "";
let firstLoad = true;
let regionList = [];
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

    syncData()
})


function syncData() {
    fetch("./tasks", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotTasks)
    fetch("./regions", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotRegions)
}

function operate(val, row) {
    return '<a class="remove" title="remove">Cancel</a>';
}


function gotTasks(data) {
    const processedData = data.map((item) => {
        let task = item.StorageTask || item.RetrievalTask
        const sched = {
            Schedule: task.Schedule,
            Limit: task.ScheduleLimit,
        }
        delete task.Schedule;
        delete task.ScheduleLimit;
        return Object.assign({ sched, task }, item)
    }).filter((item) => item.Status == 1 || item.sched.Schedule)
    if (firstLoad) {
        let stringify = (d) => JSON.stringify(d, null, 2);
        $("#taskTable").bootstrapTable({
            idField: 'UUID',
            columns: [
                {title:'ID', field:'UUID'},
                {title:'Task', field:'task', formatter: stringify},
                {title:'Schedule', field:'sched', formatter: stringify},
                {title:'Delete', field: 'operate', align: 'center', formatter: operate, events: { 'click .remove': cancel}}
            ],
            data: processedData,
        });
    } else {
        $("#taskTable").bootstrapTable('load', processedData)
    }
}

function gotRegions(data) {
    if (firstLoad) {
        regionList = data["regions"]
        $.each(regionList, (reg) => {
            let li = $('<li/>').appendTo($("#regionlist"));
            let button = $('<button/>')
                .addClass("list-group-action")
                .text(regionList[reg])
                .click((regionid) => {
                    fetch(`./regions/${regionList[reg]}`, { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotDaemons)
                })
                .appendTo(li);
        });
    }
}

function gotDaemons(data) {
    let daemons = data["daemons"]
    let stringify = (d) => JSON.stringify(d, null, 2);
    $("#botdetail").bootstrapTable({
        idField: 'id',
        columns: [
            {title:'ID', field:'id'},
            {title:'Region', field:'region'},
            {title:'Wallet', field:'wallet.address'},
            {title:'Min Cap', field:'mincap'},
            {title:'Min Fil', field:'minfil'},
        ],
        data: daemons,
		});
}

function cancel(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    let url = `./tasks/${row.UUID}`
    fetch(url, { method: "DELETE", headers: getHeaders()}).then((_) => {
        $("#taskTable").bootstrapTable('hideRow', { index })
    });
}

function getHeaders() {
    let headers = {
        'Content-Type': "application/json",
    }
    if (auth != "") {
       headers.Authorization =  `Basic ${btoa(auth)}`
    }
    return headers
}

function doSubmit(e) {
    if (e.preventDefault) {
        e.preventDefault()
    }

    // loop over miners
    let miners = $("#newMiner").val().trim().split('\n')

    let requests = []
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
        } else {
            data = {
                "Miner": miner,
                "Size": Number($('#newSize').val()),
                "StartOffset": 6152, // 3 days?
                "FastRetrieval": $('#newFast').is(':checked'),
                "Verified": $('#newVerified').is(':checked'),
                "MaxPriceAttoFIL": 20000000000,
            }
            if ($('#newRepeatRetAfterStore').is(':checked')) {
                data.RetrievalSchedule = $('#newretafterstoreSchedule').val()
                data.RetrievalScheduleLimit = $('#newretafterstoreScheduleLimit').val()
            }
        }

        if ($('#newRepeat').is(':checked')) {
            data.Schedule = $('#newSchedule').val()
            data.ScheduleLimit = $('#newScheduleLimit').val()
        }
        if ($('#newScheduleTag').val() !='') {
            data.Tag = $('#newScheduleTag').val()
        }

        requests.push(fetch(url, {method: "POST", headers: getHeaders(), body: JSON.stringify(data)}))
    }

    Promise.all(requests).then((_) => {
        $("#addDone").show()
        syncData()
    })

    return false
}
