var sessionId = ""
const chatContainer = document.getElementById('chat-history');
const messageInput = document.getElementById('user-input');

messageInput.addEventListener("keydown", function (e) {
    if (e.code === "Enter") {
        sendMessage();
    }
})
// Function to check if Chrome profile is available
function isChromeProfileAvailable() {
    return !!chrome.identity && !!chrome.identity.getAuthToken;
}

// Function to get the authentication token
function getAuthToken() {
    return new Promise((resolve, reject) => {
        chrome.identity.getAuthToken({ interactive: true }, (token) => {
            if (token) {
                resolve(token);
            } else {
                reject(new Error('Failed to get authentication token'));
            }
        });
    });
}

async function sendMessage() {
    const message = messageInput.value.trim();
    if (message !== '') {
        addMessage('user', message);
        messageInput.value = '';
        // Ask agent
        let response = "";
        let token = "";
        if (isChromeProfileAvailable()) {
            try {
                const token = await getAuthToken();
            } catch (error) {
                console.error('Error:', error);
            }
        }
        try {
            response = await askAgent(token, { "sessionId": sessionId, "message": message })
        } catch (error) {
            console.error('Error:', error);
        }
        addMessage('assistant', response);
    }
}

async function askAgent(token, payload) {
    let headers = {
        'Content-Type': 'application/json'
    }
    if (token !== "") {
        headers['Authorization'] = `Bearer ${token}`
    }
    try {
        const response = await fetch('/ask', {
            method: 'POST',
            body: JSON.stringify(payload),
            headers: headers,
        })
        const respData = await response.json();
        if (!response.ok) {
            return Promise.reject(new Error(`Server side error: ${response.Error}`));
        }
        sessionId = respData.payload.sessionId;
        console.info('response sessionID: ', sessionId)
        return respData.payload.response;
    } catch (error) {
        console.error(error);
        return Promise.reject(new Error("Failed to ask the agent"));
    }
}

function addMessage(sender, message) {
    const messageElement = document.createElement('div');
    messageElement.classList.add('message', `${sender}-message`);
    messageElement.textContent = message;
    chatContainer.appendChild(messageElement);
    chatContainer.scrollTop = chatContainer.scrollHeight; // Scroll to bottom
}