export default function formatTel() {
  return {
    tel: "",
    init() {
      this.tel = this.$el.href || "";
      if (!this.tel) {
        return;
      }
      let re = /^tel:1?(\d{3})(\d{3})(\d{4})$/.exec(this.tel);
      if (re) {
        this.tel = `(${re[1]}) ${re[2]}-${re[3]}`;
      }
    },
  };
}
