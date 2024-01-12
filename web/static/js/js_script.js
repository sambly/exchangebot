$(function(){
    $('#btn-price').click(function(e){   
        $("#list-ch-price").show();
        $("#list-ch-volume").hide();
        $("#btn-graph").hide();
	});
    $('#btn-volume').click(function(e){  
        $("#list-ch-price").hide();
        $("#list-ch-volume").show(); 
        $("#btn-graph").show();
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
            //data: JSON.stringify(e.target.innerHTML),
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                forming_tickers_list_volume(response,frame);
            },
            error: function (response) {
            },    
        });
     });

     $('.btnGraph').click(function(e){    
        e.preventDefault();
        let currentBtn = $(this);
        let allButton = $('.btnGraph');
        for (let but of allButton) {
            but.classList.remove('active');
        }
        currentBtn.addClass('active');

        if (e.target.innerText === 'Цена'){
            $('#chart-price').show()
            $('#chart-volume').hide()
        }
        if (e.target.innerText === 'Объем'){
            $('#chart-price').hide()
            $('#chart-volume').show()
        }  

        chart_volume();

     });





});

function chart_volume(){

    let dataFull;

    let request = {Pair: 'BTCUSDT',Frame: '5m'};
    $.ajax({
        url: '/getChangeDelta',
        type: 'POST',
        method: 'POST',
        data: JSON.stringify(request),
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {
            console.log(typeof(response));
            dataFull = response;
            // Default Volume 
            let dataVolume = []; 
            for (let item of dataFull) {
                dataVolume.push({time:item.Time,value:item.Volume})
            }

         

            // const dataVolume = [
            //     { time: '2016-07-18', value: 80.01 },
            //     { time: '2016-07-25', value: 80.09 },
            //     { time: '2016-08-01', value: 81.23 },

            // ]


            const chartOptions = {
                layout: {
                    textColor: 'black',
                    background: { type: 'solid', color: 'white' },
                },
                height: 500,
                //autosize:true,
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

function forming_page (pairs,changePrices,deltaFast) {

    $("#list-ch-price").show();
    $("#list-ch-volume").hide();
    $("#btn-graph").hide();
    $('#chart-price').show();


    let selectPairs = document.querySelector('#pairs'); 
    for (index in pairs) {
        let option = new Option(pairs[index],pairs[index]);
        option.setAttribute("selected","false");
        selectPairs.prepend(option)
    }

    forming_tickers_list(changePrices);
    forming_tickers_list_volume(deltaFast);

}

function forming_tickers_list(changePrices) {

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

        for (let j = 0; j < heads.length; j++) {
            let cell = row.insertCell();
            let value = changePrices[item][heads[j]]["СhangePercent"].toFixed(2);
            cell.innerHTML = value;  
            cell.setAttribute("name",heads[j]);
        }       
    };


    // выбор определенной пары
    let rows = tbody.rows;
    for (let tr of rows) { 
        tr.addEventListener("click",() => {
            let pair = tr.querySelector('[name="pair"]').innerHTML;
                 new TradingView.widget(
                 {
                    "height": "500",
                    "symbol": "BINANCE:"+pair,
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


    });
    };
  
    // Сортировка таблицы
    const pair = document.querySelectorAll(".pair-price");
    let sortDirection;
    th.forEach((col, idx) => {
        col.addEventListener("click", () => {
            sortDirection = !sortDirection;

            col.classList.add("thead-flash-once");

            const rowsArrFromNodeList = Array.from(pair);
            const filteredRows = rowsArrFromNodeList.filter(
            (item) => item.style.display != "none"
            );

            filteredRows
            .sort((a, b) => {
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
        });
    });
        
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

    // Сортировка таблицы
    const pair = document.querySelectorAll(".pair-delta");
    let sortDirection;
    th.forEach((col, idx) => {
        col.addEventListener("click", () => {
            sortDirection = !sortDirection;

            col.classList.add("thead-flash-once");

            const rowsArrFromNodeList = Array.from(pair);
            const filteredRows = rowsArrFromNodeList.filter(
            (item) => item.style.display != "none"
            );

            filteredRows
            .sort((a, b) => {
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
        });
    });



};
  



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
