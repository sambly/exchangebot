$(function(){

    $('#btn-price').click(function(e){   
        show_price_panel();  
        let pair = document.querySelector('#pairs');        
        chart_price_update(pair.value); 
	});

    $('#btn-volume').click(function(e){ 
        show_volume_panel();
        chart_volume_update();
     });
     $('#btn-trade').click(function(e){ 
        show_trade_panel();
     });

    $('#btn-volume-update').click(function(e){   
        e.preventDefault();  
        $.ajax({
            url: '/updatefull',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                forming_tickers_list_volume(response);
            },
            error: function (response) {
            },
            
        });
	});

    $('.btnFrame').click(function(e){    
        e.preventDefault();
        let currentBtn = $(this);
        let allButton = $('.btnFrame');
        for (let but of allButton) {
            but.classList.remove('active');
        }
        currentBtn.addClass('active');


        let frame = e.target.innerText;
        $.ajax({
            url: '/updateframe',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                forming_tickers_list_volume(response,frame);
                chart_volume_update();
            },
            error: function (response) {
            },    
        });
     });
});


function forming_page (pairs,marketsStat,changePrices,deltaFast) {

    show_price_panel();

    // Select pairs
    let selectPairs = document.querySelector('#pairs'); 
    let selectPairsList = document.querySelector('#pairslistOptions'); 
    for (index in pairs) {
        let option = new Option(pairs[index],pairs[index]);
        selectPairsList.prepend(option)
    } 
    selectPairs.value="BTCUSDT";

    selectPairs.addEventListener('change',(e)=>{
        let pair = document.querySelector('#pairs');
        change_pair(pair.value);
    });

    // Загаловки 24ch  Volume
    let ch24Top = document.querySelector('#ch24-top');
    ch24Top.innerHTML = (marketsStat[selectPairs.value].Ch24).toLocaleString('ru',{maximumFractionDigits:2,notation: 'compact',style: 'percent'});
    let VolumeTop = document.querySelector('#volume-top');
    VolumeTop.innerHTML = (marketsStat[selectPairs.value].Volume).toLocaleString('ru',{maximumFractionDigits:2,notation: 'compact'});

    // Выбор определенного типа графика
    let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
    checkboxes.forEach((checkbox,index)=>{
        checkbox.addEventListener('change',(e)=>{
            // Сбросить все галочки 
            checkboxes.forEach((checkboxClear,index)=>{
                checkboxClear.checked = false;
            });
            checkbox.checked = true; 
            chart_volume_update();
        }) 
    }) 

    forming_tickers_list(changePrices,marketsStat);
    forming_tickers_list_volume(deltaFast);

}



function change_pair(pair){

    let selectPairs = document.querySelector('#pairs');
    selectPairs.value=pair; 


    if($('#list-ch-price').css('display') == "block"){
        chart_price_update(pair);
    }
    if($('#list-ch-volume').css('display') == "block"){
        chart_volume_update();
    }

    update_top_data(pair);
}

function update_top_data(pair){
    $.ajax({
        url: '/updateTop',
        type: 'POST',
        method: 'POST',
        data:pair,
        cache: false,
        contentType: ' text/html; charset=utf-8',
        processData: false,
        success: function (response) {
            // Загаловки 24ch  Volume
            let ch24Top = document.querySelector('#ch24-top');
            ch24Top.innerHTML = (response.Ch24).toLocaleString('ru',{maximumFractionDigits:2,notation: 'compact'})+ ' %';
            let VolumeTop = document.querySelector('#volume-top');
            VolumeTop.innerHTML = (response.Volume).toLocaleString('ru',{maximumFractionDigits:2,notation: 'compact'});;
        },
        error: function (response) {
        },    
    });
}




function chart_price_update(pair){
    new TradingView.widget(
        {
           "height": "500",
           "symbol": "BINANCE:"+ pair,
           "interval": "15",
           "timezone": "Europe/Moscow",
           "theme": "Light",
           "style": "1",
           "locale": "ru",
           "toolbar_bg": "#f1f3f6",
           "enable_publishing": false,
           "allow_symbol_change": true,
           "container_id": "tradingview_3418f"      
       }
       );
}

function chart_volume_update(){

    let pair = document.querySelector('#pairs'); 
    let frames = document.querySelectorAll('.btnFrame');
    let frame;
    let checboxType

    let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
    checkboxes.forEach((checkbox,index)=>{
        if (checkbox.checked) {
            checboxType = checkbox.value
        }
    })

    for (let f of frames) {
        if (f.classList.contains('active')) {
            frame = f;
        }
    }

    let request = {Pair: pair.value,Frame: frame.innerText};
    $.ajax({
        url: '/getChangeDelta',
        type: 'POST',
        method: 'POST',
        data: JSON.stringify(request),
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {
            let dataFull = response;
            let dataVolume = []; 
            for (let item of dataFull) {
                dataVolume.push({time:item['Time'],value:item[checboxType]})
            }

            const chartOptions = {
                layout: {
                    textColor: 'black',
                    background: { type: 'solid', color: 'white' },
                },
                height: 500,
            }; 
            
            let chart_div = document.getElementById('chart-volume');
            chart_div.innerHTML = '';

            const chart = LightweightCharts.createChart(chart_div,chartOptions);
            chart.applyOptions({
                rightPriceScale: {
                    scaleMargins: {
                        top: 0.1,
                        bottom: 0.1,
                    },
                },
            });

            const lineSeries = chart.addLineSeries({ color: '#2962FF' });
            lineSeries.setData(dataVolume);
            chart.timeScale().fitContent();


        },
        error: function (response) {
        },    
    });






}

function forming_tickers_list(changePrices,marketsStat) {

    const heads = ['ch3m','ch15m','ch1h','ch4h'];
    const tbody = document.querySelector("#tbody-price");
    const th =  document.querySelectorAll("thead[name=thead-price] th");

    let colorSet = false
    for (var item in changePrices) {
        let row = tbody.insertRow(-1);
        if (colorSet){
            row.style.backgroundColor = 'rgb(' + 239 + ',' + 239 + ',' + 239 + ')';
        } else {
            row.style.backgroundColor = 'rgb(' + 249 + ',' + 249 + ',' + 249 + ')';
        }
        colorSet = !colorSet

        row.className = "pair-price";
        let cell = row.insertCell();
        cell.innerHTML = item; 
        cell.setAttribute("name","pair");
        
        cell = row.insertCell();
        cell.innerHTML = (marketsStat[item].Volume).toLocaleString('en-US',{maximumFractionDigits:0,notation: 'compact'}); 
        cell.setAttribute("name","ch24V"); 

        for (let j = 0; j < heads.length; j++) {
            let cell = row.insertCell();
            let value = changePrices[item][heads[j]]["СhangePercent"].toFixed(2);
            cell.innerHTML = value;  
            cell.setAttribute("name",heads[j]);
        }       
    };

    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) { 
        row.addEventListener("click",() => {
            let pair = row.querySelector('[name="pair"]').innerHTML;
            change_pair(pair);
    });
    };
  
    // Сортировка таблицы
    const tr = document.querySelectorAll(".pair-price");
    sort_table(tbody,th,tr);
        
}

function forming_tickers_list_volume(deltaFast,frame='5m') {

    const tbody = document.querySelector("#tbody-delta");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=thead-delta] th");
    

    let colorSet = false
    for (var item in deltaFast) {
        let row = tbody.insertRow(-1);
        if (colorSet){
            row.style.backgroundColor = 'rgb(' + 239 + ',' + 239 + ',' + 239 + ')';
        } else {
            row.style.backgroundColor = 'rgb(' + 249 + ',' + 249 + ',' + 249 + ')';
        }
        colorSet = !colorSet

        row.className = "pair-delta";
        let cell = row.insertCell();
        cell.innerHTML = item; 
        cell.setAttribute("name","pair"); 

        cell = row.insertCell();
        let value = deltaFast[item][frame]["Volume"].toFixed(2);
        cell.innerHTML = value;  
        cell.setAttribute("name",'volume');
        
        cell = row.insertCell();
        value = deltaFast[item][frame]["Trades"].toFixed(2);
        cell.innerHTML = value;  
        cell.setAttribute("name",'trades');   
    };

    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) { 
        row.addEventListener("click",() => {
            change_pair(row.querySelector('[name="pair"]').innerHTML);
        });
    };

    const tr = document.querySelectorAll(".pair-delta");
    // Сортировка таблицы
    sort_table(tbody,th,tr);
};

function sort_table(tbody,th,tr) {
    
    let sortDirection;
    // удалить обработчики старые 
    $(th).off();
    th.forEach((col, idx) => {
        $(col).on("click", () => {
            console.log("Тык тыгыдык");
            sortDirection = !sortDirection;
            const rowsArrFromNodeList = Array.from(tr); 
            // Первый столбец строки
            if (idx>0) {
                rowsArrFromNodeList.sort((a, b) => {
                    return a.childNodes[idx].innerHTML-b.childNodes[idx].innerHTML
                })
                .forEach((row) => {
                    sortDirection
                    ? tbody.insertBefore(row, tbody.childNodes[tbody.length])
                    : tbody.insertBefore(row, tbody.childNodes[0]);
                });

            } else {         
                    rowsArrFromNodeList.sort((a, b) => {
                    return a.childNodes[idx].innerHTML.localeCompare(
                    b.childNodes[idx].innerHTML,
                    "en",
                    { numeric: true, sensitivity: "base" }
                    );
                    })
                    .forEach((row) => {
                        sortDirection
                        ? tbody.insertBefore(row, tbody.childNodes[tbody.length])
                        : tbody.insertBefore(row, tbody.childNodes[0]);
                    });
            }
        });
    });
}

function show_price_panel(){

    $("#list-ch-price").show();
    $("#list-ch-volume").hide();

    $("#chart-price").show();
    $("#panel-chart-volume").hide();
    $("#panel-trade").hide();

}

function show_volume_panel(){
    $("#list-ch-price").hide();
    $("#list-ch-volume").show();
   
    $("#chart-price").hide();
    $("#panel-chart-volume").show();  
    $("#panel-trade").hide();
}

function show_trade_panel(){
    $("#list-ch-price").show();
    $("#list-ch-volume").hide();

    $("#chart-price").hide();
    $("#panel-chart-volume").hide();
    $("#panel-trade").show();
}


function get_response_message (response,reload) {
    if (response['err']!="" && response['err']!=null ) {
        alert(response['err']);
        return true
    } else if (response['message']!="" && response['message']!=null){
        alert(response['message']);
        if (reload) location.reload(); 
        return true 
    }
    return false
}
