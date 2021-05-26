async function onSubmit(evt) {
    evt.preventDefault();

    let itms = document.getElementById('retdat').value.trim().split(/\s+/);
    for (;itms.length>0;) {
        let miner = itms.shift();
        let cid = itms.shift();
        let resp = await fetch('/tasks/retrieval', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({"Miner":miner,"PayloadCID":cid,"CARExport":false})
        })
    }
    console.log(resp)

    return false;
}

function setupForm() {
    document.getElementsByTagName('form')[0].addEventListener('submit', onSubmit, true);
}

window.addEventListener('load',setupForm, true);