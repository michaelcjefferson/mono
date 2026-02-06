document.addEventListener('alpine:init', () => {
  Alpine.store('toast', {
    items: [],
    
    show(message, type = 'error', duration = 5000) {
      const id = Date.now();
      
      this.items.push({
        id,
        message,
        type,
        show: true
      });
      
      if (duration > 0) {
        setTimeout(() => {
          this.hide(id);
        }, duration);
      }
      
      return id;
    },
    
    hide(id) {
      const item = this.items.find(i => i.id === id);
      if (item) {
        item.show = false;
        // Remove after animation completes
        setTimeout(() => {
          this.items = this.items.filter(i => i.id !== id);
        }, 300);
      }
    },
    
    success(message, duration) {
      return this.show(message, 'success', duration);
    },
    
    error(message, duration) {
      return this.show(message, 'error', duration);
    },
    
    info(message, duration) {
      return this.show(message, 'info', duration);
    },
    
    warning(message, duration) {
      return this.show(message, 'warning', duration);
    }
  });
});