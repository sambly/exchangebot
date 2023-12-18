




$(function(){

   

});



function forming_page (pairs,changePrices) {

    let selectPairs = document.querySelector('#pairs'); 
    for (index in pairs) {
        let option = new Option(pairs[index],pairs[index]);
        option.setAttribute("selected","false");
        selectPairs.prepend(option)
    }

    forming_tickers_list(changePrices)

}

function forming_tickers_list(changePrices) {
    //console.log(changePrices);

    for (var item in changePrices){


        console.log(item)
        console.log(changePrices[item])

    }



    const heads = ['ch3m','ch15m'];
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

        row.className = item;
        let cell = row.insertCell();
        cell.innerHTML = item; 
        cell.setAttribute("name",item); 

        for (let j = 0; j < heads.length; j++) {
            let cell = row.insertCell();
            let value = changePrices[item][heads[j]]["СhangePercent"];
            cell.innerHTML = value;  
            cell.setAttribute("name",heads[j]);
        }       
    };

        
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
