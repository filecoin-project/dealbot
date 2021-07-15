import "./jquery-global";
import "bootstrap-table";
import "bootstrap-cron-picker/dist/cron-picker";
import { FilecoinNumber } from "@glif/filecoin-number"
import prettyBytes from 'pretty-bytes'

let auth = "";
let regionList = [];
window.setauth = (a) => {
    auth = a;
};

$().ready(() => {
    $('#newSchedule').cronPicker();
    $('#newretafterstoreSchedule').cronPicker();

    $("#addDone").hide();
    $("#addBotDone").hide();
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
    $("#addbot button").on('click', doCreateBot);
    $("#botlist").hide()

    fetch("./regions", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotRegions)
    syncData()
})


function syncData() {
    fetch("./tasks", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotTasks)
}

function operate(val, row) {
    return '<a class="remove" title="remove">Cancel</a>';
}

let firstLoad = true

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
        firstLoad = false
    } else {
        $("#taskTable").bootstrapTable('load', processedData)
    }
}

function gotRegions(data) {
    regionList = data["regions"]
    $.each(regionList, (reg) => {
        $('<button/>')
            .addClass("list-group-item list-group-item-action")
            .text(regionList[reg])
            .on('click', function() {
                $("#regionList .list-group-item").removeClass("active")
                $(this).addClass("active")
                fetch(`./regions/${regionList[reg]}`, { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotDaemons)
            })
            .appendTo($("#regionlist"));
    });
}

let firstDaemonsLoad = true
function gotDaemons(data) {
    const daemons = data["daemons"]
    const processedDaemons = daemons.map((daemon) => {
        let daemonData = daemon.daemon
        if (daemon.funds) {
            daemonData.balance = daemon.funds.balance
            daemonData.datacap = daemon.funds.datacap
        }
        return daemonData
    })
    if (firstDaemonsLoad) {
        let filFormatter = (d) => d ? (new FilecoinNumber(d, 'attofil')).toFil() + " FIL" : "-"
        let capFormatter = (c) => c ? prettyBytes(c) : "-"
        let capStyler = (value, row) => {
            // assume no value + no data-cap = miner making unverified deals = no styling
            if (!value && !row.mincap) {
                return {}
            }
            const valNumber = value || 0
            const minCap = row.mincap || 0
            if (valNumber < minCap) {
                return { classes: 'table-danger' }
            }
            if (valNumber - minCap < ((1 << 30)*32)) {
                return { classes: 'table-warning' }
            }
            return { classes: 'table-success' }
        }
        let filStyler = (value, row) => {
            const minBal = row.minbal || 0
            const valNumber = value || 0
            if (value < row.minbal) {
                return { classes: 'table-danger' }
            }
            if (valNumber - minBal < Math.pow(10, 18)) {
                return { classes: 'table-warning' }
            }
            return { classes: 'table-success' }
        }
        $("#botlist").show()

    $("#botdetail").bootstrapTable({
        idField: 'id',
        columns: [
            {title:'ID', field:'id'},
            {title:'Region', field:'region'},
            {title:'Wallet', field:'wallet.address'},
            {title: 'Balance', field: 'balance', formatter: filFormatter, cellStyle: filStyler},
            {title: 'Data Cap', field: 'datacap', formatter: capFormatter, cellStyle: capStyler}
        ],
        data: processedDaemons,
		});
        firstDaemonsLoad = false
    } else {
        $("#botdetail").bootstrapTable('load', processedDaemons) 
    }
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

function doCreateBot(e) {
    let region = $("#newBotRegion").val()
    let url = `./regions/${region}`

    data = {
        "id": $("#newBotId").val(),
        "tags": $("#newBotTags").val().trim().split('\n'),
        "workers": parseInt($("#newBotWorkers").val()),
        "mincap": parseInt($("#newBotMinCap").val()),
        "minfil": parseInt($("#newBotMinFil").val()),
        "wallet": {
            "address": $("#newBotWalletAddress").val(),
            "exported": $("#newBotWalletExported").val(),
        },
    }
    fetch(url, {method: "POST", headers: getHeaders(), body: JSON.stringify(data)}).then(() => {
	      $("#addBotDone").show()
    })
}
