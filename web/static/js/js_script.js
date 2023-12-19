




$(function(){
  

});


function forming_page (pairs,changePrices) {

    let selectPairs = document.querySelector('#pairs'); 
    for (index in pairs) {
        let option = new Option(pairs[index],pairs[index]);
        option.setAttribute("selected","false");
        selectPairs.prepend(option)
    }

    forming_tickers_list(changePrices);

}

function forming_tickers_list(changePrices) {

    const heads = ['ch3m','ch15m','ch1h','ch4h'];
    const tbody = document.querySelector("tbody");
    const th = document.querySelectorAll("thead th");

    let colorSet = false
    for (var item in changePrices) {
        let row = tbody.insertRow(-1);
        if (colorSet){
            row.style.backgroundColor = 'rgb(' + 239 + ',' + 239 + ',' + 239 + ')';
        } else {
            row.style.backgroundColor = 'rgb(' + 249 + ',' + 249 + ',' + 249 + ')';
        }
        colorSet = !colorSet

        row.className = "pair";
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
    const pair = document.querySelectorAll(".pair");
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
